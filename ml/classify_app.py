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

# ── Архитектура студента (1-в-1 из ноутбука) ────────────────────────

LABELS = {0: "neutral", 1: "positive", 2: "negative"}
TEACHER_NAME = "blanchefort/rubert-base-cased-sentiment"

# Гиперпараметры студента
STUDENT_HIDDEN = 312
STUDENT_LAYERS = 4
STUDENT_HEADS = 12
STUDENT_INTERMEDIATE = 1200
NUM_LABELS = 3
MAX_LEN = 128


class TinyBERTStudent(nn.Module):
    def __init__(self, teacher_cfg):
        super().__init__()
        self.bert = BertModel(BertConfig(
            vocab_size=teacher_cfg.vocab_size,
            hidden_size=STUDENT_HIDDEN,
            num_hidden_layers=STUDENT_LAYERS,
            num_attention_heads=STUDENT_HEADS,
            intermediate_size=STUDENT_INTERMEDIATE,
            max_position_embeddings=teacher_cfg.max_position_embeddings,
            type_vocab_size=teacher_cfg.type_vocab_size,
            attn_implementation="eager",
        ))
        self.embed_proj = nn.Linear(STUDENT_HIDDEN, 768)
        self.hidden_proj = nn.Linear(STUDENT_HIDDEN, 768)
        self.classifier = nn.Linear(STUDENT_HIDDEN, NUM_LABELS)

    def forward(self, input_ids, attention_mask=None, token_type_ids=None):
        out = self.bert(
            input_ids=input_ids,
            attention_mask=attention_mask,
            token_type_ids=token_type_ids,
            output_attentions=True,
            output_hidden_states=True,
        )
        cls = out.last_hidden_state[:, 0]
        return {
            "logits": self.classifier(cls),
            "hidden_states": out.hidden_states,
            "attentions": out.attentions,
            "cls": cls,
        }

# ── Инициализация ───────────────────────────────────────────────────

CHECKPOINT = "best_student.pt"
DEVICE = torch.device("cuda" if torch.cuda.is_available() else "cpu")

app = FastAPI(title="TinyBERT Sentiment Classifier")

# Токенизатор учителя (словарь тот же)
tokenizer = BertTokenizer.from_pretrained(TEACHER_NAME)

# Для создания студента нужен конфиг учителя (vocab_size и т.д.)
teacher_cfg = BertConfig.from_pretrained(TEACHER_NAME)
model = TinyBERTStudent(teacher_cfg).to(DEVICE)

# Чекпоинт — словарь {"model_state": ..., "cls_cos": ..., "acc": ...}
ckpt = torch.load(CHECKPOINT, map_location=DEVICE, weights_only=False)
model.load_state_dict(ckpt["model_state"])
model.eval()

# ── Эндпоинт ────────────────────────────────────────────────────────

@app.post("/classify", response_model=ClassifyResponse)
def classify(req: ClassifyRequest):
    if not req.text.strip():
        raise HTTPException(status_code=422, detail="text must not be empty")

    encoding = tokenizer(
        req.text,
        max_length=MAX_LEN,
        padding="max_length",
        truncation=True,
        return_tensors="pt",
    )
    encoding = {k: v.to(DEVICE) for k, v in encoding.items()}

    with torch.no_grad():
        out = model(**encoding)
        probs = F.softmax(out["logits"], dim=-1).squeeze(0)

    scores = {LABELS[i]: round(probs[i].item(), 2) for i in range(NUM_LABELS)}
    best = int(probs.argmax())

    return ClassifyResponse(
        label=LABELS[best],
        confidence=scores[LABELS[best]],
        all_scores=scores,
    )
