package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/bots/shared"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot инкапсулирует telegram-бота и зависимости.
type Bot struct {
	api    *tgbotapi.BotAPI
	client *shared.APIClient
}

func NewBot(api *tgbotapi.BotAPI, client *shared.APIClient) *Bot {
	return &Bot{api: api, client: client}
}

// Run запускает long-polling цикл.
func (b *Bot) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := b.api.GetUpdatesChan(u)
	log.Printf("Bot @%s started, waiting for messages...", b.api.Self.UserName)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		go b.handleMessage(update.Message)
	}
}

// handleMessage обрабатывает одно входящее сообщение.
func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	// Команды.
	switch {
	case msg.IsCommand():
		b.handleCommand(msg)
		return
	case text == "":
		// Игнорируем медиа и пустые сообщения.
		return
	}

	// Показываем "печатает..." пока ждём ответа от API.
	typing := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	b.api.Send(typing) //nolint:errcheck

	// Таймаут на запрос к Core API.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := b.client.Check(ctx, text)

	var replyText string
	if err != nil {
		log.Printf("[ERROR] chat=%d text=%q err=%v", chatID, text, err)
		replyText = formatError(err)
	} else {
		log.Printf("[OK] chat=%d label=%s confidence=%.2f", chatID, result.Label, result.Confidence)
		replyText = formatVerdict(result)
	}

	reply := tgbotapi.NewMessage(chatID, replyText)
	reply.ParseMode = tgbotapi.ModeMarkdown
	reply.ReplyToMessageID = msg.MessageID

	if _, err := b.api.Send(reply); err != nil {
		log.Printf("[ERROR] send reply chat=%d: %v", chatID, err)
	}
}

// handleCommand обрабатывает команды (/start, /help).
func (b *Bot) handleCommand(msg *tgbotapi.Message) {
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
	b.api.Send(reply) //nolint:errcheck
}
