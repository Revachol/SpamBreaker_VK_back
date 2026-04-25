package utils

import (
	"fmt"

	"github.com/Revachol/SpamBreaker_VK_back/internal/clients/core"
)

// labelMeta — всё для отображения одного лейбла.
type labelMeta struct {
	emoji string
	title string
}

var labels = map[string]labelMeta{
	"positive": {emoji: "🟢", title: "Позитив"},
	"neutral":  {emoji: "⚪️", title: "Нейтрально"},
	"negative": {emoji: "🔴", title: "Негатив"},
}

// FormatVerdict формирует итоговое сообщение для пользователя.
func FormatVerdict(r *check_client.CheckResponse) string {
	meta, ok := labels[r.Label]
	if !ok {
		meta = labelMeta{emoji: "❓", title: r.Label}
	}

	pct := int(r.Confidence * 100)

	// Полоска уверенности (10 делений).
	bar := ConfidenceBar(r.Confidence)

	return fmt.Sprintf(
		"%s *Вердикт: %s* (%d%%)\n\n"+
			"`%s`\n\n"+
			"*Все оценки:*\n"+
			"🟢 Позитив:    %.0f%%\n"+
			"⚪️ Нейтрально: %.0f%%\n"+
			"🔴 Негатив:    %.0f%%",
		meta.emoji,
		meta.title,
		pct,
		bar,
		r.AllScores["positive"]*100,
		r.AllScores["neutral"]*100,
		r.AllScores["negative"]*100,
	)
}

// ConfidenceBar рисует ASCII-полоску уверенности, например: [████████░░] 80%
func ConfidenceBar(confidence float64) string {
	const total = 10
	filled := int(confidence * float64(total))
	bar := ""
	for i := 0; i < total; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return fmt.Sprintf("[%s] %d%%", bar, int(confidence*100))
}

// FormatError — сообщение при ошибке.
func FormatError(err error) string {
	return fmt.Sprintf("⚠️ Не удалось проверить сообщение.\n\n`%v`", err)
}
