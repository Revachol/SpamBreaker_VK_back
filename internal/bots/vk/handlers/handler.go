package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/bots/utils"
	check_client "github.com/Revachol/SpamBreaker_VK_back/internal/clients/core"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain/expectation"
	botmetrics "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/bot"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/mail-ru-im/bot-golang"
)

// VKBot инкапсулирует VK Teams бота и зависимости.
type VKBot struct {
	api    *botgolang.Bot
	client *check_client.APIClient
	logger logger.Log
}

func NewBot(bot *botgolang.Bot, client *check_client.APIClient, l logger.Log) *VKBot {
	return &VKBot{
		api:    bot,
		client: client,
		logger: l,
	}
}

// Run запускает цикл обработки событий (long-polling).
func (b *VKBot) Run(coll botmetrics.BotMetricsIface) {
	ctx := context.Background()

	// Получаем канал обновлений
	updates := b.api.GetUpdatesChannel(ctx)

	b.logger.Infof("VK Teams Bot started, waiting for messages...")

	// Обертка для метрик (предполагаем, что сигнатура middleware подходит)
	processor := botmetrics.VkMiddleware(coll, b.handleMessage)

	for event := range updates {
		// Нас интересуют только новые сообщения
		if event.Type != botgolang.NEW_MESSAGE && event.Type != botgolang.EDITED_MESSAGE {
			continue
		}

		// Запускаем обработку в горутине для параллельности
		go func(e *botgolang.Message) {
			processor(e)
		}(event.Payload.Message())
	}
}

// handleEvent — адаптер для обработки сообщения из события.
func (b *VKBot) handleMessage(msg *botgolang.Message) error {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	if text == "" {
		return nil
	}

	// Проверка на команды
	if strings.HasPrefix(text, "/") {
		return b.handleCommand(msg)
	}

	// В VK Teams API "typing" отправляется через отдельный запрос,
	// но в базовой библиотеке прямой метод "SendChatAction" иногда не выведен.
	// Оставим это, если библиотека позволяет вызвать b.api.SendAction(chatID, botgolang.ActionTyping)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var errOut error = nil
	result, err := b.client.Check(ctx, text)

	var replyText string
	if err != nil {
		b.logger.Error("chat=%s text=%q err=%v", chatID, text, err)
		replyText = utils.FormatError(err)
		errOut = expectation.ClientRequestError
	} else {
		b.logger.Infof("chat=%s label=%s confidence=%.2f", chatID, result.Label, result.Confidence)
		replyText = utils.FormatVerdict(result)
	}

	// Отправка ответа через Reply (создает цитирование)
	if err := msg.Reply(replyText); err != nil {
		b.logger.Errorf("send reply chat=%s: %v", chatID, err)
		errOut = expectation.BotMessageAnswerError
	}

	return errOut
}

// handleCommand обрабатывает команды (/start, /help).
func (b *VKBot) handleCommand(msg *botgolang.Message) error {
	var text string
	chatID := msg.ForwardMsgID
	command := strings.ToLower(strings.Fields(msg.Text)[0])

	switch command {
	case "/start":
		text = "👋 Привет! Я проверяю тональность текста в VK Teams.\n\n" +
			"Просто напиши мне любое сообщение, и я скажу:\n" +
			"🟢 Позитив / ⚪️ Нейтрально / 🔴 Негатив\n\n" +
			"Попробуй написать что-нибудь!"

	case "/help":
		text = "ℹ️ *Как пользоваться:*\n\n" +
			"Отправь мне любой текст — я проанализирую его тональность с помощью ML-модели.\n\n" +
			"*Команды:*\n" +
			"/start — приветствие\n" +
			"/help  — эта справка"

	default:
		text = "❓ Неизвестная команда. Напиши /help для справки."
	}

	reply := b.api.NewTextMessage(chatID, text)
	if err := reply.Send(); err != nil {
		return expectation.BotCommandAnswerError
	}
	return nil
}
