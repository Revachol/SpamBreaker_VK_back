"""
FastAPI-сервис: Toxicity-степень (непрерывная) + спам-фильтр.

Тело: DeepPavlov/rubert-base-cased-conversational (frozen)
Голова: регрессионная голова токсичности из toxicity_head_{linear|mlp}.pt
        (обучена на soft-метках judge, выход — степень токсичности 0..1)
Pooling: MEAN по attention-маске (КАК ПРИ ОБУЧЕНИИ — критично!)

Формат ответа СОВМЕСТИМ со старым:
  {label, confidence, all_scores:{neutral, positive, negative}}
НО:
  - confidence теперь = НЕПРЕРЫВНАЯ степень токсичности (прямая, не перевёрнутая)
  - label = "negative" если степень >= TOX_THRESHOLD, иначе "neutral"
  - all_scores.negative = степень, neutral = 1 - степень, positive = 0.0

Inappropriateness убрана.

Эндпоинты:
  POST /classify        — одно сообщение
  POST /classify_batch  — список сообщений
  GET  /health
  GET  /metrics
"""

import torch
import torch.nn as nn
from fastapi import FastAPI, HTTPException, Response, Request
from pydantic import BaseModel as PydanticBase
from transformers import BertTokenizer, BertModel
# from spam_filter import SpamFilter
from prometheus_client import Counter, Histogram, generate_latest, CONTENT_TYPE_LATEST
import time

# ── Схемы (НЕ менялись — обратная совместимость) ────────────────────

class ClassifyRequest(PydanticBase):
    text: str

class ClassifyResponse(PydanticBase):
    label: str
    confidence: float
    all_scores: dict[str, float]

class BatchRequest(PydanticBase):
    messages: list[str]

class BatchMessageResult(PydanticBase):
    text: str
    label: str
    confidence: float
    all_scores: dict[str, float]

class BatchResponse(PydanticBase):
    results: list[BatchMessageResult]


# ── Константы ───────────────────────────────────────────────────────

BASE_MODEL = "DeepPavlov/rubert-base-cased-conversational"
MAX_LEN = 256                  # КАК ПРИ ОБУЧЕНИИ (было 128 — изменено)
CHECKPOINT = "model.pt"        # переименованный toxicity_head_mlp.pt (структура новая!)
TOX_THRESHOLD = 0.3            # порог для бинарного label
HIDDEN_DIM = 768
MLP_DIM = 128
DROPOUT = 0.3                  # значение из обучения (в eval не влияет)
DEVICE = torch.device("cuda" if torch.cuda.is_available() else "cpu")


# ── Метрики Prometheus ───────────────────────────────────────────────


# Создаём метрики (глобальные)
requests_total = Counter(
    'http_requests_total',
    'Total number of HTTP requests',
    ['method', 'path', 'status', "service"]
)
request_duration = Histogram(
    'http_request_duration_seconds',
    'Duration of HTTP requests in seconds',
    ['method', 'path', 'status', "service"],
    buckets=(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)
)


# ── Инициализация ───────────────────────────────────────────────────

app = FastAPI(title="Toxicity Severity Classifier")

tokenizer = BertTokenizer.from_pretrained(BASE_MODEL)
body = BertModel.from_pretrained(BASE_MODEL).to(DEVICE)
body.eval()
for p in body.parameters():
    p.requires_grad = False


def build_head(head_type: str) -> nn.Module:
    """Строит голову ТОЧНО как в ноутбуке Фазы 2 (make_head),
    чтобы ключи state_dict совпали."""
    if head_type == "linear":
        return nn.Sequential(nn.Linear(HIDDEN_DIM, 1))
    elif head_type == "mlp":
        return nn.Sequential(
            nn.Linear(HIDDEN_DIM, MLP_DIM),
            nn.ReLU(),
            nn.Dropout(DROPOUT),
            nn.Linear(MLP_DIM, 1),
        )
    raise ValueError(f"unknown head_type: {head_type}")


ckpt = torch.load(CHECKPOINT, map_location=DEVICE, weights_only=False)
head_type = ckpt.get("head_type", "mlp")
toxicity_head = build_head(head_type).to(DEVICE)
toxicity_head.load_state_dict(ckpt["state_dict"])
toxicity_head.eval()

# Защита: пулинг должен быть mean (под него обучена голова)
assert ckpt.get("pooling", "mean") == "mean", \
    "Чекпойнт обучен НЕ на mean-pooling — инференс будет неверным!"

print(f"Model loaded on {DEVICE}")
print(f"Head type: {head_type}")
print(f"CV (linear/mlp): {ckpt.get('cv_linear','N/A')}/{ckpt.get('cv_mlp','N/A')}")
print(f"Hold-out Spearman: {ckpt.get('holdout_spearman','N/A')}")
print(f"TOX_THRESHOLD={TOX_THRESHOLD}, MAX_LEN={MAX_LEN}")

spam = None  # SpamFilter()  # ВРЕМЕННО ОТКЛЮЧЁН — вернуть позже


# ── Хелперы ─────────────────────────────────────────────────────────

def _mean_pool(last_hidden: torch.Tensor, mask: torch.Tensor) -> torch.Tensor:
    """Mean-pooling по attention-маске — ИДЕНТИЧНО обучению."""
    m = mask.unsqueeze(-1).float()
    return (last_hidden * m).sum(1) / m.sum(1).clamp(min=1e-9)


def _tokenize(text: str) -> dict[str, torch.Tensor]:
    enc = tokenizer(
        text, max_length=MAX_LEN,
        padding="max_length", truncation=True, return_tensors="pt",
    )
    return {k: v.to(DEVICE) for k, v in enc.items()}


def _toxicity_score(text: str) -> float:
    """Непрерывная степень токсичности 0..1."""
    enc = _tokenize(text)
    with torch.no_grad():
        out = body(**enc)
        emb = _mean_pool(out.last_hidden_state, enc["attention_mask"])
        score = torch.sigmoid(toxicity_head(emb)).squeeze().item()
    return float(score)


def _classify_text(text: str) -> dict:
    # ── Спам-фильтр ВРЕМЕННО ОТКЛЮЧЁН (вернуть вместе со spam_filter) ──
    # spam_result = spam.check(text)
    # if spam_result["is_spam"]:
    #     conf = round(spam_result["confidence"], 4)
    #     return {
    #         "label": "negative",
    #         "confidence": conf,
    #         "all_scores": {
    #             "neutral": round(1 - conf, 4),
    #             "positive": 0.0,
    #             "negative": conf,
    #         },
    #     }

    # Степень токсичности (ПРЯМАЯ, не перевёрнутая)
    score = _toxicity_score(text)
    label = "negative" if score >= TOX_THRESHOLD else "neutral"

    sc = round(score, 4)
    return {
        "label": label,
        "confidence": sc,                      # = степень токсичности
        "all_scores": {
            "neutral": round(1 - sc, 4),
            "positive": 0.0,
            "negative": sc,
        },
    }


# ── Эндпоинты ───────────────────────────────────────────────────────

@app.post("/classify", response_model=ClassifyResponse)
def classify(req: ClassifyRequest):
    if not req.text.strip():
        raise HTTPException(status_code=422, detail="text must not be empty")
    return ClassifyResponse(**_classify_text(req.text))


@app.post("/classify_batch", response_model=BatchResponse)
def classify_batch(req: BatchRequest):
    if not req.messages:
        raise HTTPException(status_code=422, detail="messages must not be empty")
    results = []
    for text in req.messages:
        if text.strip():
            data = _classify_text(text)
            data["text"] = text
            results.append(BatchMessageResult(**data))
    return BatchResponse(results=results)


@app.get("/health")
def health():
    return {"status": "ok", "device": str(DEVICE), "head": head_type}

@app.get("/metrics")
def metrics():
    return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)
