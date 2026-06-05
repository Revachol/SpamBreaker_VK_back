package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	check_client "github.com/Revachol/SpamBreaker_VK_back/internal/clients/core"
	botmetrics "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/bot"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// checkResult – упрощённая модель результата проверки текста.
type checkResult struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

// Bot инкапсулирует telegram-бота и зависимости.
type Bot struct {
	api    *tgbotapi.BotAPI
	client *check_client.APIClient
	logger logger.Log
}

func NewBot(api *tgbotapi.BotAPI, client *check_client.APIClient, l logger.Log) *Bot {
	return &Bot{api: api, client: client, logger: l}
}

// Run запускает цикл обработки обновлений (long-polling).
func (b *Bot) Run(coll botmetrics.BotMetricsIface) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := b.api.GetUpdatesChan(u)
	b.logger.Infof("Bot @%s started, waiting for updates...", b.api.Self.UserName)
	processor := botmetrics.TgMiddleware(coll, func(msg *tgbotapi.Message) error {
		// метрика только для сообщений, my_chat_member обрабатывается отдельно
		return nil
	})

	for update := range updates {
		// Обработка my_chat_member (бот добавлен в чат)
		if update.MyChatMember != nil {
			b.handleChatAdded(update.MyChatMember)
			continue
		}

		if update.Message == nil {
			continue
		}
		msg := update.Message

		// Для сообщений с текстом применяем метрику
		if msg.Text != "" {
			go func(m *tgbotapi.Message) {
				processor(m)
			}(msg)
		}
		b.handleMessage(msg)
	}
}

// spamThreshold – минимальная уверенность для удаления сообщения как спам.
const spamThreshold = 0.70

// handleMessage обрабатывает одно входящее сообщение.
func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	if msg.IsCommand() {
		b.handleCommand(msg)
		return
	}
	if text == "" {
		return
	}

	// Групповой чат: проверяем регистрацию и анализируем текст
	if msg.Chat.IsGroup() || msg.Chat.IsSuperGroup() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		result, err := b.client.CheckMessage(ctx, text, strconv.FormatInt(chatID, 10))
		if err != nil {
			b.logger.Errorf("chat=%d text=%q check error: %v", chatID, text, err)
			return
		}

		b.logger.Infof("chat=%d label=%s confidence=%.2f", chatID, result.Label, result.Confidence)

		if result.Label == "negative" && result.Confidence >= spamThreshold {
			// Удаляем сообщение
			del := tgbotapi.NewDeleteMessage(chatID, msg.MessageID)
			if _, err := b.api.Request(del); err != nil {
				b.logger.Errorf("delete message chat=%d msg=%d: %v", chatID, msg.MessageID, err)
			}
			sender := "пользователь"
			if msg.From != nil {
				if msg.From.UserName != "" {
					sender = "@" + msg.From.UserName
				} else {
					sender = msg.From.FirstName
				}
			}
			notice := tgbotapi.NewMessage(chatID, fmt.Sprintf(
				"🚫 Сообщение от %s удалено: обнаружен спам (%.0f%%)",
				sender, result.Confidence*100,
			))
			b.api.Send(notice) //nolint:errcheck
		}
	}
}

// handleMyChatMember обрабатывает событие добавления бота в чат.
func (b *Bot) handleChatAdded(update *tgbotapi.ChatMemberUpdated) {
	// Реагируем только на добавление бота (status = "member")
	if update.NewChatMember.Status != "member" {
		return
	}
	chatID := update.Chat.ID
	fromUser := update.From

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := b.client.ActivateChat(ctx, chatID, fromUser.ID)
	if err != nil {
		b.logger.Warnf("VerifyAddChat failed for chat=%d, user=%d: %v", chatID, fromUser.ID, err)
		// Отправляем предупреждение в чат и выходим
		warn := tgbotapi.NewMessage(chatID,
			"⛔ Бот может быть добавлен только верифицированным администратором. Пожалуйста, подтвердите аккаунт в приложении и убедитесь, что вы администратор этой группы.")
		b.api.Send(warn) //nolint:errcheck
		if _, err := b.api.Request(tgbotapi.LeaveChatConfig{ChatID: chatID}); err != nil {
			b.logger.Warnf("LeaveChat failed for chat=%d: %v", chatID, err)
		}
		return
	}

	// Успех – приветственное сообщение
	welcome := tgbotapi.NewMessage(chatID,
		"✅ Бот SpamBreaker успешно активирован! Я буду автоматически модерировать сообщения.\n"+
			"Настройки доступны в личном кабинете.")
	b.api.Send(welcome) //nolint:errcheck
}

// handleCommand обрабатывает команды (/start, /help, /connect).
func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	var text string

	switch msg.Command() {
	case "connect":
		b.handleConnect(msg)
		return

	case "start":
		text = "👋 Привет! Я SpamBreaker – бот для модерации групп.\n\n" +
			"Чтобы подключить меня к группе:\n" +
			"1. Зарегистрируйтесь на сайте и подтвердите аккаунт (получите код в личном кабинете и отправьте его мне в личку).\n" +
			"2. Добавьте меня в группу как администратора.\n" +
			"3. Отправьте в группе команду `/connect ВАШ_ТОКЕН`.\n\n" +
			"*Команды:*\n" +
			"/connect TOKEN — активировать бота в этой группе\n" +
			"/help — справка"

	case "help":
		text = "ℹ️ *Как подключить бота к группе:*\n\n" +
			"1. Подтвердите свой аккаунт в личном кабинете → получите код верификации.\n" +
			"2. Отправьте этот код мне в личные сообщения.\n" +
			"3. Добавьте бота в группу как администратора.\n" +
			"4. Выполните в группе `/connect ВАШ_ТОКЕН` (токен можно скопировать в личном кабинете).\n\n" +
			"После этого бот начнёт модерировать сообщения."

	default:
		text = "❓ Неизвестная команда. Напиши /help для справки."
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = tgbotapi.ModeMarkdown
	b.api.Send(reply) //nolint:errcheck
}

// handleConnect обрабатывает команду /connect TOKEN в группе.
func (b *Bot) handleConnect(msg *tgbotapi.Message) {
	if !msg.Chat.IsPrivate() {
		return
	}
	token := strings.TrimSpace(msg.CommandArguments())
	if token == "" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Укажите токен: `/connect ВАШ_ТОКЕН`")
		reply.ParseMode = tgbotapi.ModeMarkdown
		b.api.Send(reply) //nolint:errcheck
		return
	}

	userID := msg.From.ID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := b.client.VerifyUser(ctx, token, userID)
	if err != nil {
		b.logger.Errorf("Verify account failed: chat=%s user=%d token=%s err=%v", userID, token, err)
		// Ошибка может быть из-за неверифицированного пользователя, неверного токена и т.п.
		reply := tgbotapi.NewMessage(msg.Chat.ID,
			"❌ Не удалось активировать аккаунт. Убедитесь, что вы являетесь владельцем токена.")
		b.api.Send(reply) //nolint:errcheck
		return
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID,
		"✅ *Ваш аккаунт SpamBreaker успешно активирован!*")
	reply.ParseMode = tgbotapi.ModeMarkdown
	b.api.Send(reply) //nolint:errcheck
}

// isHex проверяет, состоит ли строка только из шестнадцатеричных символов.
func isHex(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
