package botmetrics

import (
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Middleware оборачивает функцию обработчика обновлений, собирая метрики.
// Принимает имя бота, коллектор и функцию обработки обновлений (обычно бот.Request).
func Middleware(
	collector BotMetricsIface,
	next func(message *tgbotapi.Message) error,
) func(msg *tgbotapi.Message) {
	return func(msg *tgbotapi.Message) {
		start := time.Now()
		command := extractCommand(msg)

		err := next(msg)

		status := "success"
		if err != nil {
			status = err.Error()
		}

		duration := time.Since(start).Seconds()
		collector.IncBotRequest(command, status)
		collector.ObserveBotDuration(command, status, duration)
	}
}

// extractCommand извлекает команду или тип обновления.
func extractCommand(msg *tgbotapi.Message) string {
	if msg.IsCommand() {
		// возвращаем команду с префиксом "/", например "/start"
		return msg.Command()
	}
	// тип сообщения (текст, фото, документ и т.д.)
	switch {
	case msg.Text != "":
		return "text"
	case msg.Photo != nil:
		return "photo"
	case msg.Document != nil:
		return "document"
	case msg.Sticker != nil:
		return "sticker"
	case msg.Voice != nil:
		return "voice"
	case msg.Video != nil:
		return "video"
	case msg.Location != nil:
		return "location"
	default:
		return "other"
	}
}
