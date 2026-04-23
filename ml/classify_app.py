"""
FastAPI-сервис для Toxicity + Inappropriateness классификации.

Тело: DeepPavlov/rubert-base-cased-conversational (загружается из HuggingFace)
Головы: toxicity MLP + inappropriateness Linear (загружаются из model.pt)

Выход бинарный:
  negative = toxic (>0.6) или inappropriate (>0.5)
  positive = всё остальное

Два эндпоинта:
  POST /classify       — одно сообщение
  POST /classify_batch — список сообщений
"""

import torch
import torch.nn as nn
import torch.nn.functional as F
from fastapi import FastAPI, HTTPException, Response, Request
from pydantic import BaseModel as PydanticBase
from transformers import BertTokenizer, BertModel
import transformers
from prometheus_client import Counter, Histogram, generate_latest, CONTENT_TYPE_LATEST
import time

transformers.utils.import_utils._torch_version = "2.6.0"
# ── Схема запроса / ответа ──────────────────────────────────────────

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


# ── Константы ────────────────────────────────────────────────────────

BASE_MODEL = "DeepPavlov/rubert-base-cased-conversational"
MAX_LEN = 128
CHECKPOINT = "model.pt"
TOX_THRESHOLD = 0.75
INAPP_THRESHOLD = 0.5
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

service = "Model"
app = FastAPI(title="Toxicity Classifier")

tokenizer = BertTokenizer.from_pretrained(BASE_MODEL)
body = BertModel.from_pretrained(BASE_MODEL).to(DEVICE)
body.eval()
for p in body.parameters():
    p.requires_grad = False

# Загрузка голов
ckpt = torch.load(CHECKPOINT, map_location=DEVICE, weights_only=False)

toxicity_head = nn.Sequential(
    nn.Linear(ckpt["hidden_dim"], ckpt["tox_mlp_dim"]),
    nn.ReLU(),
    nn.Dropout(ckpt["tox_dropout"]),
    nn.Linear(ckpt["tox_mlp_dim"], 2),
).to(DEVICE)
toxicity_head.load_state_dict(ckpt["toxicity_head"])
toxicity_head.eval()

inapprop_head = nn.Linear(ckpt["hidden_dim"], 2).to(DEVICE)
inapprop_head.load_state_dict(ckpt["inapprop_head"])
inapprop_head.eval()

print(f"Model loaded on {DEVICE}")
print(f"Toxicity F1: {ckpt.get('toxicity_f1', 'N/A')}")
print(f"Inapprop F1: {ckpt.get('inapprop_f1', 'N/A')}")


# ── Хелперы ──────────────────────────────────────────────────────────

def _tokenize(text: str) -> dict[str, torch.Tensor]:
    enc = tokenizer(
        text, max_length=MAX_LEN,
        padding="max_length", truncation=True, return_tensors="pt",
    )
    return {k: v.to(DEVICE) for k, v in enc.items()}


def _classify_text(text: str) -> dict:
    enc = _tokenize(text)
    with torch.no_grad():
        cls = body(**enc).pooler_output

        tox_probs = F.softmax(toxicity_head(cls), dim=-1).squeeze(0)
        p_toxic = tox_probs[1].item()

        inapp_probs = F.softmax(inapprop_head(cls), dim=-1).squeeze(0)
        p_inapp = inapp_probs[1].item()

    is_negative = p_toxic > TOX_THRESHOLD or (p_toxic <= TOX_THRESHOLD and p_inapp > INAPP_THRESHOLD)

    if is_negative:
        confidence = max(p_toxic, p_inapp)
        label = "negative"
    else:
        confidence = 1 - max(p_toxic, p_inapp)
        label = "neutral"

    neg_score = round(max(p_toxic, p_inapp), 4)
    neut_score = round(1 - neg_score, 4)

    return {
        "label": label,
        "confidence": round(confidence, 4),
        "all_scores": {
            "neutral": neut_score,
            "positive": 0.0,
            "negative": neg_score,
        },
    }


# ── Эндпоинты ────────────────────────────────────────────────────────


@app.middleware("http")
async def metrics_middleware(request: Request, call_next):
    start = time.time()
    response = await call_next(request)
    duration = time.time() - start

    endpoint = request.url.path
    method = request.method
    status = response.status_code

    if endpoint == "/health" or endpoint == "/metrics":
        return response
    requests_total.labels(method=method, path=endpoint, status=status, service=service).inc()
    request_duration.labels(method=method, path=endpoint, status=status, service=service).observe(duration)

    return response


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
    return {"status": "ok", "device": str(DEVICE)}


@app.get("/metrics")
def metrics():
    return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)
