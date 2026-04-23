package bot

import (
	"context"
	"strconv"
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

	if msg.IsCommand() {
		return b.handleCommand(msg)
	}
	if text == "" {
		return nil
	}

	// Для групповых чатов проверяем, что чат зарегистрирован в системе.
	if msg.Chat.IsGroup() || msg.Chat.IsSuperGroup() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		active, err := b.client.IsChatActive(ctx, strconv.FormatInt(chatID, 10))
		cancel()
		if err != nil {
			b.logger.Warnf("failed to check chat %d registration: %v", chatID, err)
			return nil
		}
		if !active {
			return nil
		}
	}

	typing := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	b.api.Send(typing) //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var errOut error
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

// handleCommand обрабатывает команды (/start, /help, /connect).
func (b *Bot) handleCommand(msg *tgbotapi.Message) error {
	var text string

	switch msg.Command() {
	case "connect":
		return b.handleConnect(msg)

	case "start":
		text = "👋 Привет! Я SpamBreaker — бот для модерации групп.\n\n" +
			"Чтобы подключить меня к группе, зарегистрируйтесь на сайте и следуйте инструкции.\n\n" +
			"*Команды:*\n" +
			"/connect TOKEN — привязать бота к группе\n" +
			"/help — справка"

	case "help":
		text = "ℹ️ *Как подключить бота к группе:*\n\n" +
			"1. Зарегистрируйтесь на сайте SpamBreaker\n" +
			"2. Перейдите в раздел Telegram и скопируйте токен\n" +
			"3. Добавьте бота в группу как администратора\n" +
			"4. Отправьте в группе команду: `/connect ВАШ_ТОКЕН`\n\n" +
			"После этого бот начнёт модерировать сообщения."

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

// handleConnect обрабатывает команду /connect TOKEN.
func (b *Bot) handleConnect(msg *tgbotapi.Message) error {
	token := strings.TrimSpace(msg.CommandArguments())
	if token == "" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Укажите токен: `/connect ВАШ_ТОКЕН`")
		reply.ParseMode = tgbotapi.ModeMarkdown
		b.api.Send(reply) //nolint:errcheck
		return nil
	}

	chatID := strconv.FormatInt(msg.Chat.ID, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.client.ActivateChat(ctx, token, chatID); err != nil {
		b.logger.Errorf("failed to activate chat %s with token: %v", chatID, err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Не удалось подключить бота. Проверьте токен и попробуйте снова.")
		b.api.Send(reply) //nolint:errcheck
		return err
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, "✅ *Бот SpamBreaker успешно подключён!*\n\nМодерация сообщений активирована.")
	reply.ParseMode = tgbotapi.ModeMarkdown
	b.api.Send(reply) //nolint:errcheck
	return nil
}
