package botmetrics

import (
	"strings"
	"time"

	"github.com/SevereCloud/vksdk/v2/object"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TgMiddleware оборачивает функцию обработчика обновлений, собирая метрики.
// Принимает имя бота, коллектор и функцию обработки обновлений (обычно бот.Request).
func TgMiddleware(
	collector BotMetricsIface,
	next func(message *tgbotapi.Message) error,
) func(msg *tgbotapi.Message) {
	return func(msg *tgbotapi.Message) {
		start := time.Now()
		command := extractTgCommand(msg)

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
func extractTgCommand(msg *tgbotapi.Message) string {
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

// VkMiddleware оборачивает функцию обработчика обновлений, собирая метрики.
// Принимает имя бота, коллектор и функцию обработки обновлений (обычно бот.Request).
func VkMiddleware(
	collector BotMetricsIface,
	next func(message *object.MessagesMessage) error,
) func(msg *object.MessagesMessage) {
	return func(msg *object.MessagesMessage) {
		start := time.Now()
		command := extractVkCommand(msg)

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
func extractVkCommand(msg *object.MessagesMessage) string {
	// 1. Проверка на текстовые команды
	if strings.HasPrefix(msg.Text, "/") {
		// Возвращаем первое слово (саму команду), например "/start"
		return strings.Fields(msg.Text)[0]
	}

	// 2. Если есть вложения, определяем тип по первому вложению
	if len(msg.Attachments) > 0 {
		attType := msg.Attachments[0].Type
		switch attType {
		case "audio_message": // В ВК голосовые сообщения — это audio_message
			return "voice"
		case "doc":
			return "file"
		case "photo":
			return "photo"
		case "video":
			return "video"
		case "audio":
			return "audio"
		default:
			return "attachment"
		}
	}

	// 3. Если вложений нет, но есть текст
	if msg.Text != "" {
		return "text"
	}

	return "other"
}
