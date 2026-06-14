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
		// Обработка my_chat_member (бот добавлен, получил права администратора или удалён из чата)
		b.logger.Tracef("Update: %v", update)
		if update.MyChatMember != nil {
			b.handleChatMemberChanged(update.MyChatMember)
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

	if msg.NewChatTitle != "" {
		b.handleChatRenamed(msg)
		return
	}

	if msg.IsCommand() {
		b.handleCommand(msg)
		return
	}
	if text == "" {
		return
	}

	// Групповой чат: проверяем регистрацию и анализируем текст
	if !(msg.Chat.IsGroup() || msg.Chat.IsSuperGroup()) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := b.client.CheckMessage(ctx, text, strconv.FormatInt(chatID, 10), strconv.Itoa(msg.MessageID))
	if err != nil {
		b.logger.Errorf("chat=%d text=%q check error: %v", chatID, text, err)
		return
	}
	if result == nil {
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

// handleChatRenamed обрабатывает событие переименования чата или группы.
func (b *Bot) handleChatRenamed(msg *tgbotapi.Message) {
	if !msg.Chat.IsGroup() && !msg.Chat.IsSuperGroup() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.client.UpdateChatName(ctx, msg.Chat.ID, msg.NewChatTitle); err != nil {
		b.logger.Warnf("UpdateChatName failed for chat=%d name=%q: %v", msg.Chat.ID, msg.NewChatTitle, err)
	}
}

// handleChatMemberChanged обрабатывает изменение статуса самого бота в чате.
func (b *Bot) handleChatMemberChanged(update *tgbotapi.ChatMemberUpdated) {
	oldStatus := update.OldChatMember.Status
	newStatus := update.NewChatMember.Status

	switch {
	case isBotJoinedChat(oldStatus, newStatus):
		b.handleChatAdded(update)
	case isBotPromoted(oldStatus, newStatus) && update.NewChatMember.IsAdministrator():
		b.handleChatPromoted(update)
	case isBotRemoved(newStatus):
		b.handleChatRemoved(update)
	}
}

func isBotJoinedChat(oldStatus, newStatus string) bool {
	return (oldStatus == "left" || oldStatus == "kicked") && (newStatus == "member" || newStatus == "administrator")
}

func isBotPromoted(oldStatus, newStatus string) bool {
	return oldStatus != "administrator" && newStatus == "administrator"
}

func isBotDemoted(oldStatus, newStatus string) bool {
	return oldStatus == "administrator" && newStatus == "member"
}

func isBotRemoved(newStatus string) bool {
	return newStatus == "left" || newStatus == "kicked"
}

// handleChatPromoted обрабатывает выдачу боту прав администратора.
func (b *Bot) handleChatPromoted(update *tgbotapi.ChatMemberUpdated) {
	chatID := update.Chat.ID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.client.ActivateChat(ctx, chatID); err != nil {
		b.logger.Warnf("ActivateAddedChat failed for chat=%d: %v", chatID, err)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "✅ Права администратора получены. Модерация сообщений активирована.")
	b.api.Send(msg) //nolint:errcheck
}

// handleChatAdded обрабатывает событие добавления бота в чат.
func (b *Bot) handleChatAdded(update *tgbotapi.ChatMemberUpdated) {
	if update.NewChatMember.Status != "member" && update.NewChatMember.Status != "administrator" {
		return
	}
	chatID := update.Chat.ID
	fromUser := update.From

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	member, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: fromUser.ID,
		},
	})
	if err != nil {
		b.logger.Warnf("GetChatMember failed for chat=%d, user=%d: %v", chatID, fromUser.ID, err)
	}
	if !(member.IsAdministrator() || member.IsCreator()) {
		// Отправляем предупреждение в чат и выходим
		warn := tgbotapi.NewMessage(chatID,
			"⛔ Бот может быть добавлен только верифицированным администратором. Пожалуйста, подтвердите аккаунт в приложении и убедитесь, что вы администратор этой группы.")
		b.api.Send(warn) //nolint:errcheck
		if _, err := b.api.Request(tgbotapi.LeaveChatConfig{ChatID: chatID}); err != nil {
			b.logger.Warnf("LeaveChat failed for chat=%d: %v", chatID, err)
		}
		return
	}

	err = b.client.ActivateAddChat(ctx, update.Chat.Title, fromUser.ID, chatID)
	if err != nil {
		b.logger.Warnf("VerifyAddChat failed for chat=%d, user=%d: %v", chatID, fromUser.ID, err)
		return
	}

	welcome := tgbotapi.NewMessage(chatID,
		"⚠️ Для работы SpamBreaker нужны права администратора.\n"+
			"Пожалуйста, выдайте мне права администратора в настройках группы, чтобы я мог модерировать сообщения.")
	b.api.Send(welcome) //nolint:errcheck

	if update.NewChatMember.Status == "administrator" {
		b.handleChatPromoted(update)
		return
	}
}

// handleChatRemoved обрабатывает событие удаления бота из чата.
func (b *Bot) handleChatRemoved(update *tgbotapi.ChatMemberUpdated) {
	chatID := update.Chat.ID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.client.DeactivateChat(ctx, chatID); err != nil {
		b.logger.Warnf("DeactivateChat failed for chat=%d: %v", chatID, err)
	}
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
