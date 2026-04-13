package utils

import (
	"context"
	"strings"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/clients/telegram"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain/expectation"
	botmetrics "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/bot"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot инкапсулирует telegram-бота и зависимости.
type Bot struct {
	api    *tgbotapi.BotAPI
	client *telegram.APIClient
	logger logger.Log
}

func NewBot(api *tgbotapi.BotAPI, client *telegram.APIClient, l logger.Log) *Bot {
	return &Bot{api: api, client: client, logger: l}
}

// Run запускает long-polling цикл.
func (b *Bot) Run(coll botmetrics.BotMetricsIface) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := b.api.GetUpdatesChan(u)
	b.logger.Infof("Bot @%s started, waiting for messages...", b.api.Self.UserName)
	processor := botmetrics.Middleware(coll, b.handleMessage)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		go processor(update.Message)
	}
}

// handleMessage обрабатывает одно входящее сообщение.
func (b *Bot) handleMessage(msg *tgbotapi.Message) error {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	// Команды.
	switch {
	case msg.IsCommand():
		return b.handleCommand(msg)
	case text == "":
		// Игнорируем медиа и пустые сообщения.
		return nil
	}

	// Показываем "печатает..." пока ждём ответа от API.
	typing := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	b.api.Send(typing) //nolint:errcheck

	// Таймаут на запрос к Core API.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var errOut error = nil
	result, err := b.client.Check(ctx, text)

	var replyText string
	if err != nil {
		b.logger.Error("chat=%d text=%q err=%v", chatID, text, err)
		replyText = formatError(err)
		errOut = expectation.ClientRequestError
	} else {
		b.logger.Infof("chat=%d label=%s confidence=%.2f", chatID, result.Label, result.Confidence)
		replyText = formatVerdict(result)
	}

	reply := tgbotapi.NewMessage(chatID, replyText)
	reply.ParseMode = tgbotapi.ModeMarkdown
	reply.ReplyToMessageID = msg.MessageID

	if _, err := b.api.Send(reply); err != nil {
		b.logger.Errorf("send reply chat=%d: %v", chatID, err)
		errOut = expectation.BotMessageAnswerError
	}
	return errOut
}

// handleCommand обрабатывает команды (/start, /help).
func (b *Bot) handleCommand(msg *tgbotapi.Message) error {
	var text string

	switch msg.Command() {
	case "start":
		text = "👋 Привет! Я проверяю тональность текста.\n\n" +
			"Просто напиши мне любое сообщение, и я скажу:\n" +
			"🟢 Позитив / ⚪️ Нейтрально / 🔴 Негатив\n\n" +
			"Попробуй написать что-нибудь!"

	case "help":
		text = "ℹ️ *Как пользоваться:*\n\n" +
			"Отправь мне любой текст — я проанализирую его тональность с помощью ML-модели.\n\n" +
			"*Команды:*\n" +
			"/start — приветствие\n" +
			"/help  — эта справка"

	default:
		text = "❓ Неизвестная команда. Напиши /help для справки."
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = tgbotapi.ModeMarkdown
	if _, err := b.api.Send(reply); err != nil {
		return expectation.BotCommandAnswerError
	}
	return nil
}
