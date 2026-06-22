package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/bots/utils"
	check_client "github.com/Revachol/SpamBreaker_VK_back/internal/clients/core"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain/expectation"
	botmetrics "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/bot"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/api/params"
	"github.com/SevereCloud/vksdk/v2/events"
	"github.com/SevereCloud/vksdk/v2/longpoll-bot"
	"github.com/SevereCloud/vksdk/v2/object"
)

const spamThreshold = 0.70

type VKBot struct {
	vk     *api.VK
	lp     *longpoll.LongPoll
	client *check_client.APIClient
	logger logger.Log
}

func NewBot(vk *api.VK, lp *longpoll.LongPoll, client *check_client.APIClient, l logger.Log) *VKBot {

	return &VKBot{
		vk:     vk,
		lp:     lp,
		client: client,
		logger: l,
	}
}

func (b *VKBot) Run(coll botmetrics.BotMetricsIface) {
	b.logger.Infof("VK Bot started...")

	// Middleware для метрик
	processor := botmetrics.VkMiddleware(coll, b.handleMessage)

	// Обработка новых сообщений
	b.lp.MessageNew(func(ctx context.Context, obj events.MessageNewObject) {
		msg := obj.Message
		processor(&msg)
	})

	// Запуск прослушивания
	if err := b.lp.Run(); err != nil {
		b.logger.Fatalf("Long Poll run error: %v", err)
	}
}

func (b *VKBot) handleMessage(msg *object.MessagesMessage) error {
	// В ВК PeerID > 2000000000 означает групповой чат
	isGroup := msg.PeerID > 2000000000
	text := strings.TrimSpace(msg.Text)
	peerIDStr := strconv.Itoa(msg.PeerID)

	if b.handleChatAction(msg) {
		return nil
	}

	if text == "" {
		return nil
	}

	// Обработка команд
	if strings.HasPrefix(text, "/") {
		return b.handleCommand(msg)
	}

	if !isGroup {
		return nil
	}
	// Анализ текста через ML Core
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := b.client.CheckMessage(
		ctx,
		text,
		peerIDStr,
		strconv.Itoa(msg.ConversationMessageID),
		time.UnixMilli(int64(msg.Date)),
	)
	if err != nil {
		b.sendMessage(msg.PeerID, utils.FormatError(err), msg.ConversationMessageID)
		b.logger.Errorf("VK Check error: %v", err)
		return expectation.ClientRequestError
	}
	if result == nil {
		return nil
	}
	b.logger.Debugf("VK Check result: %v", result)

	if result.Label == "negative" && result.Confidence >= spamThreshold {
		// Удаление сообщения в ВК
		_, err := b.vk.MessagesDelete(api.Params{
			"peer_id":        msg.PeerID,
			"cmids":          msg.ConversationMessageID,
			"delete_for_all": 1,
			"group_id":       b.lp.GroupID,
		})
		if err != nil {
			b.logger.Errorf("failed to delete msg: %v", err)
		}

		notice := fmt.Sprintf("🚫 Сообщение удалено: обнаружен спам (%.0f%%)", result.Confidence*100)
		b.sendMessage(msg.PeerID, notice, 0)
	}

	return nil
}

func (b *VKBot) handleChatAction(msg *object.MessagesMessage) bool {
	b.logger.Tracef("VK ChatActionType: %v", msg.
		Action.Type)
	switch msg.Action.Type {
	case object.ChatTitleUpdate:
		b.handleChatRenamed(msg)
		return true
	case object.ChatInviteUser, object.ChatInviteUserByLink:
		if msg.Action.MemberID == b.botMemberID() {
			b.handleChatAdded(msg)
			b.handleChatPromoted(msg.PeerID)
			return true
		}
	case object.ChatKickUser:
		if msg.Action.MemberID == b.botMemberID() {
			b.handleChatRemoved(msg)
			return true
		}
	}

	return false
}

func (b *VKBot) handleChatRenamed(msg *object.MessagesMessage) {
	if msg.PeerID <= 2000000000 || msg.Action.Text == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.client.UpdateChatName(ctx, int64(msg.PeerID), msg.Action.Text); err != nil {
		b.logger.Warnf("UpdateChatName failed for chat=%d name=%q: %v", msg.PeerID, msg.Action.Text, err)
	}
}

func (b *VKBot) handleChatAdded(msg *object.MessagesMessage) {
	chatID := msg.PeerID
	fromUserID := msg.FromID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.client.ActivateAddChat(ctx, vkChatName(msg), int64(fromUserID), int64(chatID)); err != nil {
		b.logger.Warnf("ActivateAddChat failed for chat=%d, user=%d: %v", chatID, fromUserID, err)
		return
	}

	b.sendMessage(chatID,
		"⚠️ Для работы SpamBreaker нужны права администратора.\n"+
			"Пожалуйста, выдайте мне права администратора в настройках беседы, чтобы я мог модерировать сообщения.",
		0,
	)
}

func (b *VKBot) handleChatPromoted(chatID int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.client.ActivateChat(ctx, int64(chatID)); err != nil {
		b.logger.Warnf("ActivateChat failed for chat=%d: %v", chatID, err)
		return
	}

	b.sendMessage(chatID, "✅ Права администратора получены. Модерация сообщений активирована.", 0)
}

func (b *VKBot) handleChatRemoved(msg *object.MessagesMessage) {
	chatID := msg.PeerID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.client.DeactivateChat(ctx, int64(chatID)); err != nil {
		b.logger.Warnf("DeactivateChat failed for chat=%d: %v", chatID, err)
	}
}

func (b *VKBot) handleCommand(msg *object.MessagesMessage) error {
	parts := strings.Fields(msg.Text)
	cmd := strings.ToLower(parts[0])

	var response string
	switch cmd {
	case "/start":
		response = "👋 Привет! Я SpamBreaker для ВКонтакте.\n\n" +
			"Команды:\n" +
			"/connect TOKEN — подтвердить аккаунт\n" +
			"/help — справка"
	case "/help":
		response = "ℹ️ Как подключить VK-бота:\n\n" +
			"1. Подтвердите аккаунт в личном кабинете и получите код верификации.\n" +
			"2. Отправьте этот код мне в личные сообщения командой /connect ВАШ_ТОКЕН.\n" +
			"3. Добавьте меня в беседу.\n" +
			"4. Выдайте мне права администратора, чтобы я мог модерировать сообщения."
	case "/connect":
		return b.handleConnect(msg, parts)
	default:
		response = "❓ Неизвестная команда."
	}

	b.sendMessage(msg.PeerID, response, msg.ConversationMessageID)
	return nil
}

// Вспомогательный метод для отправки сообщений
func (b *VKBot) sendMessage(peerID int, text string, replyTo int) {
	p := params.NewMessagesSendBuilder()
	p.PeerID(peerID)
	p.RandomID(0) // 0 позволяет SDK сгенерировать случайное число
	p.Message(text)

	if replyTo != 0 {
		// В ВК для ответа используется conversation_message_ids
		p.Forward(fmt.Sprintf(`{"is_reply":1,"peer_id":%d,"conversation_message_ids":[%d]}`, peerID, replyTo))
	}

	_, err := b.vk.MessagesSend(p.Params)
	if err != nil {
		b.logger.Errorf("failed to send message to %d: %v", peerID, err)
	}
}

// handleConnect обрабатывает команду /connect TOKEN.
func (b *VKBot) handleConnect(msg *object.MessagesMessage, parts []string) error {
	if msg.PeerID > 2000000000 {
		return nil
	}
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		b.sendMessage(msg.PeerID, "❌ Укажите токен: /connect ВАШ_ТОКЕН", msg.ConversationMessageID)
		return nil
	}

	token := strings.TrimSpace(parts[1])

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.client.VerifyUser(ctx, token, int64(msg.FromID)); err != nil {
		b.logger.Errorf("Verify account failed: user=%d token=%s err=%v", msg.FromID, token, err)
		b.sendMessage(msg.PeerID, "❌ Не удалось активировать аккаунт. Убедитесь, что вы являетесь владельцем токена.", msg.ConversationMessageID)
		return nil
	}

	b.sendMessage(msg.PeerID, "✅ Ваш аккаунт SpamBreaker успешно активирован!", msg.ConversationMessageID)

	return nil
}

func (b *VKBot) botMemberID() int {
	return -b.lp.GroupID
}

func vkChatName(msg *object.MessagesMessage) string {
	if msg.Action.Text != "" {
		return msg.Action.Text
	}
	return fmt.Sprintf("VK Chat %d", msg.PeerID)
}
