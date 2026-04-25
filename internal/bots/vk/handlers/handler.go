package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/bots/utils"
	check_client "github.com/Revachol/SpamBreaker_VK_back/internal/clients/core"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain/expectation"
	botmetrics "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/bot"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/mail-ru-im/bot-golang"
)

// spamThreshold — минимальная уверенность для удаления сообщения как спам.
const spamThreshold = 0.70

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

func (b *VKBot) Run(coll botmetrics.BotMetricsIface) {
	ctx := context.Background()
	updates := b.api.GetUpdatesChannel(ctx)

	b.logger.Infof("VK Teams Bot @%s started", b.api.Info.Nick)

	processor := botmetrics.VkMiddleware(coll, b.handleMessage)

	for event := range updates {
		if event.Type != botgolang.NEW_MESSAGE && event.Type != botgolang.EDITED_MESSAGE {
			continue
		}

		msg := event.Payload.Message()
		if msg == nil {
			continue
		}

		go func(m *botgolang.Message) {
			processor(m)
		}(msg)
	}
}

func (b *VKBot) handleMessage(msg *botgolang.Message) error {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	// Проверка на команды
	if strings.HasPrefix(text, "/") {
		return b.handleCommand(msg)
	}

	if text == "" {
		return nil
	}

	// В VK Teams типами групповых чатов являются "group" и "channel"
	isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "channel"

	// Для групповых чатов проверяем регистрацию в системе.
	if isGroup {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		active, err := b.client.IsChatActive(ctx, chatID) // chatID в VK уже string
		cancel()
		if err != nil {
			b.logger.Warnf("failed to check chat %s registration: %v", chatID, err)
			return nil
		}
		if !active {
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := b.client.Check(ctx, text)
	if err != nil {
		b.logger.Errorf("chat=%s text=%q err=%v", chatID, text, err)
		if !isGroup {
			msg.Reply(utils.FormatError(err))
		}
		return expectation.ClientRequestError
	}

	b.logger.Infof("chat=%s label=%s confidence=%.2f", chatID, result.Label, result.Confidence)

	if isGroup {
		// Групповой чат: удаляем спам (negative), чистые сообщения игнорируем.
		if result.Label == "negative" && result.Confidence >= spamThreshold {
			if err := msg.Delete(); err != nil {
				b.logger.Errorf("delete message chat=%s msg=%s: %v", chatID, msg.Chat.Nick, err)
			}

			sender := "пользователь"
			if msg.Chat.Nick != "" {
				sender = "@" + msg.Chat.Nick
			} else if msg.Chat.FirstName != "" {
				sender = msg.Chat.FirstName
			}

			notice := fmt.Sprintf("🚫 Сообщение от %s удалено: обнаружен спам (%.0f%%)",
				sender, result.Confidence*100)

			// Отправляем уведомление в чат
			b.api.NewTextMessage(chatID, notice).Send()
		}
		return nil
	}

	// Личный чат: отправляем вердикт (цитированием)
	if err := msg.Reply(utils.FormatVerdict(result)); err != nil {
		b.logger.Errorf("send reply chat=%s: %v", chatID, err)
		return expectation.BotMessageAnswerError
	}

	return nil
}

func (b *VKBot) handleCommand(msg *botgolang.Message) error {
	parts := strings.Fields(msg.Text)
	command := strings.ToLower(parts[0])

	switch command {
	case "/connect":
		return b.handleConnect(msg, parts)

	case "/start":
		text := "👋 Привет! Я SpamBreaker — бот для модерации групп VK Teams.\n\n" +
			"Чтобы подключить меня к группе, зарегистрируйтесь на сайте и следуйте инструкции.\n\n" +
			"*Команды:*\n" +
			"/connect TOKEN — привязать бота к группе\n" +
			"/help — справка"
		msg.Reply(text)

	case "/help":
		text := "ℹ️ *Как подключить бота к группе:*\n\n" +
			"1. Зарегистрируйтесь на сайте SpamBreaker\n" +
			"2. Перейдите в раздел VK Teams и скопируйте токен\n" +
			"3. Добавьте бота в группу как администратора\n" +
			"4. Отправьте в группе команду: `/connect ВАШ_ТОКЕН`"
		msg.Reply(text)

	default:
		msg.Reply("❓ Неизвестная команда. Напиши /help для справки.")
	}

	return nil
}

func (b *VKBot) handleConnect(msg *botgolang.Message, parts []string) error {
	if len(parts) < 2 {
		msg.Reply("❌ Укажите токен: `/connect ВАШ_ТОКЕН`")
		return nil
	}

	token := parts[1]
	chatID := msg.Chat.ID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.client.ActivateChat(ctx, token, chatID); err != nil {
		b.logger.Errorf("failed to activate chat %s with token: %v", chatID, err)
		msg.Reply("❌ Не удалось подключить бота. Проверьте токен и попробуйте снова.")
		return err
	}

	msg.Reply("✅ *Бот SpamBreaker успешно подключён!*\n\nМодерация сообщений активирована.")
	return nil
}
