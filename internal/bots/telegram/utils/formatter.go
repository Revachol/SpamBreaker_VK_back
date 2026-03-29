package utils

import (
	"fmt"

	"github.com/Revachol/SpamBreaker_VK_back/internal/clients/telegram"
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

// formatVerdict формирует итоговое сообщение для пользователя.
func formatVerdict(r *telegram.CheckResponse) string {
	meta, ok := labels[r.Label]
	if !ok {
		meta = labelMeta{emoji: "❓", title: r.Label}
	}

	pct := int(r.Confidence * 100)

	// Полоска уверенности (10 делений).
	bar := confidenceBar(r.Confidence)

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

// confidenceBar рисует ASCII-полоску уверенности, например: [████████░░] 80%
func confidenceBar(confidence float64) string {
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

// formatError — сообщение при ошибке.
func formatError(err error) string {
	return fmt.Sprintf("⚠️ Не удалось проверить сообщение.\n\n`%v`", err)
}
