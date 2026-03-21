"""
FastAPI-сервис для ContextSentimentModel
(TinyBERT + CrossAttention + TimeAwareGRU).

Два эндпоинта:
  POST /classify       — одно сообщение (без контекста, delta=0)
  POST /classify_scene — цепочка сообщений с delta_seconds между ними
"""

import math
from typing import Optional

import torch
import torch.nn as nn
import torch.nn.functional as F
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel as PydanticBase
from transformers import BertModel, BertConfig, BertTokenizer

# ── Схема запроса / ответа ──────────────────────────────────────────

class ClassifyRequest(PydanticBase):
    text: str

class ClassifyResponse(PydanticBase):
    label: str
    confidence: float
    all_scores: dict[str, float]


class SceneMessage(PydanticBase):
    text: str
    delta_seconds: float = 0.0      # пауза ДО этого сообщения (сек.)


class SceneRequest(PydanticBase):
    messages: list[SceneMessage]


class SceneMessageResult(PydanticBase):
    text: str
    label: str
    confidence: float
    all_scores: dict[str, float]
    delta_seconds: float


class SceneResponse(PydanticBase):
    results: list[SceneMessageResult]


# ── Константы ────────────────────────────────────────────────────────

LABELS = {0: "neutral", 1: "positive", 2: "negative"}
TEACHER_NAME = "blanchefort/rubert-base-cased-sentiment"

# Гиперпараметры (1-в-1 из ноутбука)
HIDDEN = 312
NUM_LAYERS = 4
NUM_HEADS = 12
INTERMEDIATE = 1200
CONTEXT_DIM = 312
CROSS_ATTN_HEADS = 4
CROSS_ATTN_LAYERS = 2
NUM_LABELS = 3
MAX_LEN = 128

CHECKPOINT = "best_context_model.pt"
DEVICE = torch.device("cuda" if torch.cuda.is_available() else "cpu")

# ── Архитектура (1-в-1 из ноутбука) ─────────────────────────────────


class CrossAttentionBlock(nn.Module):
    """
    N слоёв cross-attention поверх hidden states.
    Применяется AFTER BERT, не внутри.
    """

    def __init__(
        self,
        hidden_size: int,
        num_heads: int = 4,
        num_layers: int = 2,
        dropout: float = 0.1,
    ):
        super().__init__()
        self.layers = nn.ModuleList()
        for _ in range(num_layers):
            self.layers.append(nn.ModuleDict({
                "q": nn.Linear(hidden_size, hidden_size),
                "k": nn.Linear(hidden_size, hidden_size),
                "v": nn.Linear(hidden_size, hidden_size),
                "out": nn.Linear(hidden_size, hidden_size),
                "norm": nn.LayerNorm(hidden_size),
                "dropout": nn.Dropout(dropout),
            }))
        self.num_heads = num_heads
        self.head_dim = hidden_size // num_heads

    def forward(self, hidden_states: torch.Tensor, context: torch.Tensor):
        ctx = context.unsqueeze(1)              # (B, 1, H)
        B, S, H = hidden_states.shape

        for layer in self.layers:
            residual = hidden_states

            Q = layer["q"](hidden_states).view(B, S, self.num_heads, self.head_dim).transpose(1, 2)
            K = layer["k"](ctx).view(B, 1, self.num_heads, self.head_dim).transpose(1, 2)
            V = layer["v"](ctx).view(B, 1, self.num_heads, self.head_dim).transpose(1, 2)

            attn = torch.matmul(Q, K.transpose(-2, -1)) / math.sqrt(self.head_dim)
            attn = torch.softmax(attn, dim=-1)
            attn = layer["dropout"](attn)

            out = torch.matmul(attn, V)
            out = out.transpose(1, 2).contiguous().view(B, S, H)
            out = layer["out"](out)
            out = layer["dropout"](out)

            hidden_states = layer["norm"](residual + out)

        return hidden_states


class TimeAwareGRU(nn.Module):
    """GRU + временной гейт: большая пауза → сильнее обновление контекста."""

    def __init__(self, hidden_size: int):
        super().__init__()
        self.gru_cell = nn.GRUCell(hidden_size, hidden_size)
        self.time_gate = nn.Linear(hidden_size * 2 + 1, hidden_size)

    def forward(self, context, cls_vec, delta_seconds):
        gru_out = self.gru_cell(cls_vec, context)
        delta_feat = torch.log(delta_seconds.unsqueeze(-1) + 1)
        gate_input = torch.cat([context, cls_vec, delta_feat], dim=-1)
        gamma = torch.sigmoid(self.time_gate(gate_input))
        return (1 - gamma) * context + gamma * gru_out


class ContextSentimentModel(nn.Module):
    def __init__(self, teacher_cfg: BertConfig):
        super().__init__()

        self.bert = BertModel(BertConfig(
            vocab_size=teacher_cfg.vocab_size,
            hidden_size=HIDDEN,
            num_hidden_layers=NUM_LAYERS,
            num_attention_heads=NUM_HEADS,
            intermediate_size=INTERMEDIATE,
            max_position_embeddings=teacher_cfg.max_position_embeddings,
            type_vocab_size=teacher_cfg.type_vocab_size,
            attn_implementation="eager",
        ))

        self.cross_attn = CrossAttentionBlock(
            HIDDEN,
            num_heads=CROSS_ATTN_HEADS,
            num_layers=CROSS_ATTN_LAYERS,
        )
        self.context_gru = TimeAwareGRU(CONTEXT_DIM)
        self.classifier = nn.Linear(HIDDEN + CONTEXT_DIM, NUM_LABELS)

        # Проекции из дистилляции (нужны для загрузки чекпоинта)
        self.embed_proj = nn.Linear(HIDDEN, 768)
        self.hidden_proj = nn.Linear(HIDDEN, 768)

    def encode_message(self, input_ids, attention_mask, token_type_ids, context):
        bert_out = self.bert(
            input_ids=input_ids,
            attention_mask=attention_mask,
            token_type_ids=token_type_ids,
        )
        hidden_states = bert_out.last_hidden_state
        enriched = self.cross_attn(hidden_states, context)
        return enriched[:, 0]                     # CLS

    def predict_message(self, input_ids, attention_mask, token_type_ids, context, delta):
        """Инференс одного сообщения → logits + обновлённый контекст."""
        cls = self.encode_message(input_ids, attention_mask, token_type_ids, context)
        combined = torch.cat([cls, context], dim=-1)
        logits = self.classifier(combined)
        new_context = self.context_gru(context, cls, delta)
        return logits, new_context


# ── Инициализация ───────────────────────────────────────────────────

app = FastAPI(title="ContextSentiment Classifier")

tokenizer = BertTokenizer.from_pretrained(TEACHER_NAME)
teacher_cfg = BertConfig.from_pretrained(TEACHER_NAME)

model = ContextSentimentModel(teacher_cfg).to(DEVICE)

ckpt = torch.load(CHECKPOINT, map_location=DEVICE, weights_only=False)
model.load_state_dict(ckpt["model_state"])
model.eval()

# ── Хелперы ──────────────────────────────────────────────────────────


def _tokenize(text: str) -> dict[str, torch.Tensor]:
    enc = tokenizer(
        text,
        max_length=MAX_LEN,
        padding="max_length",
        truncation=True,
        return_tensors="pt",
    )
    return {k: v.to(DEVICE) for k, v in enc.items()}


def _scores_from_logits(logits: torch.Tensor) -> tuple[dict[str, float], int]:
    probs = F.softmax(logits, dim=-1).squeeze(0)
    scores = {LABELS[i]: round(probs[i].item(), 4) for i in range(NUM_LABELS)}
    best = int(probs.argmax())
    return scores, best


# ── Эндпоинты ────────────────────────────────────────────────────────


@app.post("/classify", response_model=ClassifyResponse)
def classify(req: ClassifyRequest):
    """Классификация одного сообщения (контекст = нулевой вектор)."""
    if not req.text.strip():
        raise HTTPException(status_code=422, detail="text must not be empty")

    enc = _tokenize(req.text)
    context = torch.zeros(1, CONTEXT_DIM, device=DEVICE)
    delta = torch.tensor([0.0], device=DEVICE)

    with torch.no_grad():
        logits, _ = model.predict_message(
            enc["input_ids"], enc["attention_mask"],
            enc.get("token_type_ids", torch.zeros_like(enc["input_ids"])),
            context, delta,
        )

    scores, best = _scores_from_logits(logits)
    return ClassifyResponse(
        label=LABELS[best],
        confidence=scores[LABELS[best]],
        all_scores=scores,
    )


@app.post("/classify_scene", response_model=SceneResponse)
def classify_scene(req: SceneRequest):
    """
    Классификация цепочки сообщений с учётом контекста.

    Пример тела запроса:
    {
      "messages": [
        {"text": "Привет!", "delta_seconds": 0},
        {"text": "Дурак ты)", "delta_seconds": 3},
        {"text": "Сам такой", "delta_seconds": 2}
      ]
    }
    """
    if not req.messages:
        raise HTTPException(status_code=422, detail="messages must not be empty")

    context = torch.zeros(1, CONTEXT_DIM, device=DEVICE)
    results: list[SceneMessageResult] = []

    with torch.no_grad():
        for msg in req.messages:
            if not msg.text.strip():
                continue
            enc = _tokenize(msg.text)
            delta = torch.tensor([msg.delta_seconds], dtype=torch.float, device=DEVICE)

            logits, context = model.predict_message(
                enc["input_ids"], enc["attention_mask"],
                enc.get("token_type_ids", torch.zeros_like(enc["input_ids"])),
                context, delta,
            )

            scores, best = _scores_from_logits(logits)
            results.append(SceneMessageResult(
                text=msg.text,
                label=LABELS[best],
                confidence=scores[LABELS[best]],
                all_scores=scores,
                delta_seconds=msg.delta_seconds,
            ))

    return SceneResponse(results=results)
