

"""
FastAPI-сервис для TinyBERT Sentiment + Toxicity.

Два эндпоинта:
  POST /classify       — одно сообщение → sentiment + toxicity
  POST /classify_batch — список сообщений → sentiment + toxicity для каждого
"""

import torch
import torch.nn as nn
import torch.nn.functional as F
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel as PydanticBase
from transformers import BertConfig, BertTokenizer
from transformers.models.bert.modeling_bert import BertEncoder, BertPooler

# ── Схема запроса / ответа ──────────────────────────────────────────

class ClassifyRequest(PydanticBase):
    text: str

class ClassifyResponse(PydanticBase):
    text: str
    sentiment: str
    sentiment_confidence: float
    sentiment_scores: dict[str, float]
    toxic: bool
    toxicity_confidence: float
    toxicity_scores: dict[str, float]

class BatchRequest(PydanticBase):
    messages: list[str]

class BatchResponse(PydanticBase):
    results: list[ClassifyResponse]


# ── Константы ────────────────────────────────────────────────────────

SENT_LABELS = {0: "neutral", 1: "positive", 2: "negative"}
TOX_LABELS = {0: "safe", 1: "toxic"}
TEACHER_NAME = "blanchefort/rubert-base-cased-sentiment"
MAX_LEN = 128
CHECKPOINT = "model.pt"
DEVICE = torch.device("cuda" if torch.cuda.is_available() else "cpu")


# ── Архитектура ──────────────────────────────────────────────────────

class FactorizedEmbedding(nn.Module):
    def __init__(self, vocab_size, embed_dim, hidden_dim, pad_token_id=0,
                 max_position_embeddings=512, type_vocab_size=2, layer_norm_eps=1e-12):
        super().__init__()
        self.word_embeddings = nn.Embedding(vocab_size, embed_dim, padding_idx=pad_token_id)
        self.position_embeddings = nn.Embedding(max_position_embeddings, embed_dim)
        self.token_type_embeddings = nn.Embedding(type_vocab_size, embed_dim)
        self.projection = nn.Linear(embed_dim, hidden_dim)
        self.LayerNorm = nn.LayerNorm(hidden_dim, eps=layer_norm_eps)
        self.dropout = nn.Dropout(0.1)
        self.register_buffer("position_ids", torch.arange(max_position_embeddings).unsqueeze(0))

    def forward(self, input_ids, token_type_ids=None):
        seq_len = input_ids.size(1)
        position_ids = self.position_ids[:, :seq_len]
        if token_type_ids is None:
            token_type_ids = torch.zeros_like(input_ids)
        embeddings = (
            self.word_embeddings(input_ids)
            + self.position_embeddings(position_ids)
            + self.token_type_embeddings(token_type_ids)
        )
        embeddings = self.projection(embeddings)
        embeddings = self.LayerNorm(embeddings)
        embeddings = self.dropout(embeddings)
        return embeddings


class TinyBERTStudent(nn.Module):
    def __init__(self, vocab_size, embed_dim=128, hidden_dim=512,
                 num_layers=6, num_heads=8, intermediate_size=2048,
                 num_labels=3, max_position_embeddings=512, pad_token_id=0):
        super().__init__()
        self.hidden_dim = hidden_dim
        self.num_labels = num_labels

        self.embeddings = FactorizedEmbedding(
            vocab_size=vocab_size, embed_dim=embed_dim,
            hidden_dim=hidden_dim, pad_token_id=pad_token_id,
            max_position_embeddings=max_position_embeddings,
        )

        encoder_config = BertConfig(
            vocab_size=vocab_size, hidden_size=hidden_dim,
            num_hidden_layers=num_layers, num_attention_heads=num_heads,
            intermediate_size=intermediate_size,
            max_position_embeddings=max_position_embeddings,
            hidden_dropout_prob=0.1, attention_probs_dropout_prob=0.1,
        )
        self.encoder = BertEncoder(encoder_config)
        self.pooler = BertPooler(encoder_config)

        self.dropout = nn.Dropout(0.1)
        self.classifier = nn.Linear(hidden_dim, num_labels)

    def forward(self, input_ids, attention_mask=None, token_type_ids=None):
        if attention_mask is None:
            attention_mask = torch.ones_like(input_ids)
        extended_mask = attention_mask.unsqueeze(1).unsqueeze(2)
        extended_mask = (1.0 - extended_mask.float()) * -10000.0

        hidden_states = self.embeddings(input_ids, token_type_ids)
        encoder_out = self.encoder(hidden_states, attention_mask=extended_mask)
        sequence_output = encoder_out.last_hidden_state
        pooled_output = self.pooler(sequence_output)
        pooled_output = self.dropout(pooled_output)
        logits = self.classifier(pooled_output)
        return {"logits": logits, "cls_hidden": pooled_output}


# ── Инициализация ───────────────────────────────────────────────────

app = FastAPI(title="TinyBERT Sentiment + Toxicity")

tokenizer = BertTokenizer.from_pretrained(TEACHER_NAME)

# Загрузка модели
ckpt = torch.load(CHECKPOINT, map_location=DEVICE, weights_only=False)
model_config = ckpt["config"]

model = TinyBERTStudent(
    vocab_size=model_config["vocab_size"],
    embed_dim=model_config["embed_dim"],
    hidden_dim=model_config["hidden_dim"],
    num_layers=model_config["num_layers"],
    num_heads=model_config["num_heads"],
    intermediate_size=model_config["intermediate_size"],
    num_labels=model_config["num_labels"],
    pad_token_id=tokenizer.pad_token_id,
).to(DEVICE)

# Toxicity head
model.toxicity_head = nn.Linear(model_config["hidden_dim"], 2).to(DEVICE)

# Загрузка весов
model.load_state_dict(ckpt["model_state_dict"])
model.eval()

print(f"Model loaded on {DEVICE}")
print(f"Sentiment F1: {ckpt.get('sentiment_f1', 'N/A')}")
print(f"Toxicity F1:  {ckpt.get('toxicity_f1', 'N/A')}")


# ── Хелперы ──────────────────────────────────────────────────────────

def _tokenize(text: str) -> dict[str, torch.Tensor]:
    enc = tokenizer(
        text, max_length=MAX_LEN,
        padding="max_length", truncation=True, return_tensors="pt",
    )
    return {k: v.to(DEVICE) for k, v in enc.items()}


def _classify_text(text: str) -> ClassifyResponse:
    enc = _tokenize(text)
    with torch.no_grad():
        out = model(**enc)

        # Sentiment
        s_probs = F.softmax(out["logits"], dim=-1).squeeze(0)
        s_id = int(s_probs.argmax())
        s_scores = {SENT_LABELS[i]: round(s_probs[i].item(), 4) for i in range(3)}

        # Toxicity
        t_probs = F.softmax(model.toxicity_head(out["cls_hidden"]), dim=-1).squeeze(0)
        t_id = int(t_probs.argmax())
        t_scores = {TOX_LABELS[i]: round(t_probs[i].item(), 4) for i in range(2)}

    return ClassifyResponse(
        text=text,
        sentiment=SENT_LABELS[s_id],
        sentiment_confidence=s_scores[SENT_LABELS[s_id]],
        sentiment_scores=s_scores,
        toxic=(t_id == 1),
        toxicity_confidence=t_scores[TOX_LABELS[t_id]],
        toxicity_scores=t_scores,
    )


# ── Эндпоинты ────────────────────────────────────────────────────────

@app.post("/classify", response_model=ClassifyResponse)
def classify(req: ClassifyRequest):
    """Классификация одного сообщения → sentiment + toxicity."""
    if not req.text.strip():
        raise HTTPException(status_code=422, detail="text must not be empty")
    return _classify_text(req.text)


@app.post("/classify_batch", response_model=BatchResponse)
def classify_batch(req: BatchRequest):
    """Классификация списка сообщений."""
    if not req.messages:
        raise HTTPException(status_code=422, detail="messages must not be empty")
    results = [_classify_text(text) for text in req.messages if text.strip()]
    return BatchResponse(results=results)


@app.get("/health")
def health():
    return {
        "status": "ok",
        "device": str(DEVICE),
        "sentiment_f1": ckpt.get("sentiment_f1"),
        "toxicity_f1": ckpt.get("toxicity_f1"),
    }
