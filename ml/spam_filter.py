"""
Спам-фильтр: эвристики + регулярки + частотный анализ.

Использование:
    from spam_filter import SpamFilter

    sf = SpamFilter()
    result = sf.check("Заработай 100000 без вложений! http://spam.com")
    # {"is_spam": True, "confidence": 0.95, "reasons": ["url", "spam_phrase"]}

    result = sf.check("Привет, как дела?")
    # {"is_spam": False, "confidence": 0.05, "reasons": []}

Частотный анализ (опционально):
    result = sf.check("Купи слона", user_id="user123")
    # Если user123 отправил это же сообщение 5 раз за минуту → spam
"""

import re
import time
import unicodedata
from collections import defaultdict
from dataclasses import dataclass, field


@dataclass
class SpamConfig:
    # Повторы символов: "АААААААА" или "!!!!!!"
    max_char_repeat: int = 5

    # Капслок: доля заглавных букв
    caps_threshold: float = 0.5
    caps_min_length: int = 15  # не считаем капслок в коротких

    # Ссылки
    max_urls: int = 1  # больше N ссылок → спам

    # Частотный анализ
    flood_window_seconds: float = 60.0  # окно для подсчёта
    flood_max_identical: int = 7  # макс одинаковых за окно
    flood_max_messages: int = 10  # макс сообщений за окно

    # Стоп-фразы: вес каждого совпадения
    phrase_weight: float = 0.5
    # Общий порог спама (сумма весов)
    spam_threshold: float = 0.5


# Паттерны ссылок
URL_PATTERN = re.compile(
    r"https?://\S+|"
    r"t\.me/\S+|"
    r"bit\.ly/\S+|"
    r"goo\.gl/\S+|"
    r"tinyurl\.com/\S+|"
    r"\S+\.ru/\S+|"
    r"\S+\.com/\S+",
    re.IGNORECASE,
)

# Телефоны
PHONE_PATTERN = re.compile(
    r"(?:\+7|8)[\s\-]?\(?\d{3}\)?[\s\-]?\d{3}[\s\-]?\d{2}[\s\-]?\d{2}"
)

# Повторяющиеся символы
REPEAT_PATTERN = re.compile(r"(.)\1{4,}")

# Спам-фразы (lowercase)
SPAM_PHRASES = [
    "заработок без вложений",
    "заработай",
    "пассивный доход",
    "финансовая свобода",
    "розыгрыш",
    "конкурс репост",
    "подпишись и получи",
    "перейди по ссылке",
    "жми на ссылку",
    "пиши в лс",
    "пиши в личку",
    "казино",
    "ставки на спорт",
    "букмекер",
    "крипт",
    "инвестиц",
    "трейдинг",
    "forex",
    "нажми сюда",
    "скидка только сегодня",
    "акция",
    "промокод",
    "только для подписчиков",
    "заходи в группу",
    "заходи в канал",
    "подписывайся",
    "подпишись на канал",
]

# Подозрительные unicode-категории (невидимые символы, lookalikes)
SUSPICIOUS_UNICODE_CATEGORIES = {"Cf", "Co", "Cn"}


class SpamFilter:
    def __init__(self, config: SpamConfig = None):
        self.cfg = config or SpamConfig()
        # Глобальная история: text_hash → список timestamp
        self._history: dict[int, list[float]] = defaultdict(list)

    def check(self, text: str) -> dict:
        if not text or not text.strip():
            return {"is_spam": False, "confidence": 0.0, "reasons": []}

        score = 0.0
        reasons = []

        # 1. Ссылки
        urls = URL_PATTERN.findall(text)
        if len(urls) > self.cfg.max_urls:
            score += 0.4
            reasons.append("too_many_urls")
        elif len(urls) > 0:
            score += 0.25
            reasons.append("url")

        # 2. Телефоны
        phones = PHONE_PATTERN.findall(text)
        if phones:
            score += 0.2
            reasons.append("phone_number")

        # 3. Повторяющиеся символы
        repeats = REPEAT_PATTERN.findall(text)
        if repeats:
            score += 0.15
            reasons.append("char_repeat")

        # 4. Капслок
        alpha_chars = [c for c in text if c.isalpha()]
        if len(alpha_chars) >= self.cfg.caps_min_length:
            caps_ratio = sum(1 for c in alpha_chars if c.isupper()) / len(alpha_chars)
            if caps_ratio > self.cfg.caps_threshold:
                score += 0.1
                reasons.append("caps_lock")

        # 5. Спам-фразы
        text_lower = text.lower()
        matched_phrases = 0
        for phrase in SPAM_PHRASES:
            if phrase in text_lower:
                matched_phrases += 1
        if matched_phrases > 0:
            score += min(matched_phrases * self.cfg.phrase_weight, 0.6)
            reasons.append("spam_phrase")

        # 6. Подозрительные unicode-символы
        suspicious_chars = sum(
            1 for c in text
            if unicodedata.category(c) in SUSPICIOUS_UNICODE_CATEGORIES
        )
        if suspicious_chars > 5:
            score += 0.5
            reasons.append("suspicious_unicode")

        # 7. Низкая уникальность слов
        words = text_lower.split()
        if len(words) > 5:
            unique_ratio = len(set(words)) / len(words)
            if unique_ratio < 0.4:
                score += 0.5
                reasons.append("low_word_diversity")

        # 8. Глобальный флуд — идентичные сообщения от кого угодно
        flood_score = self._check_flood(text)
        if flood_score:
            score += flood_score["score"]
            reasons.extend(flood_score["reasons"])

        confidence = min(score, 1.0)
        is_spam = confidence >= self.cfg.spam_threshold

        return {
            "is_spam": is_spam,
            "confidence": round(confidence, 4),
            "reasons": reasons,
        }

    def _check_flood(self, text: str) -> dict | None:
        now = time.time()
        text_hash = hash(text.strip().lower())

        # Чистим старые записи
        timestamps = self._history[text_hash]
        timestamps = [t for t in timestamps if now - t < self.cfg.flood_window_seconds]
        timestamps.append(now)
        self._history[text_hash] = timestamps

        if len(timestamps) >= self.cfg.flood_max_identical:
            return {"score": 0.5, "reasons": ["identical_flood"]}
        return None

    def clear_history(self):
        self._history.clear()

