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

	if text == "" {
		return nil
	}

	// Обработка команд
	if strings.HasPrefix(text, "/") {
		return b.handleCommand(msg)
	}

	// Проверка активации для групп
	//if isGroup {
	//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//	active, err := b.client.IsChatActive(ctx, peerIDStr)
	//	cancel()
	//	if err != nil {
	//		b.logger.Warnf("failed to check chat %s registration: %v", peerIDStr, err)
	//		return nil
	//	}
	//	if !active {
	//		return nil
	//	}
	//}

	// Анализ текста через ML Core
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := b.client.CheckMessage(ctx, text, peerIDStr, strconv.Itoa(msg.ConversationMessageID))
	if err != nil {
		if !isGroup {
			b.sendMessage(msg.PeerID, utils.FormatError(err), msg.ConversationMessageID)
		}
		b.logger.Errorf("VK Check error: %v", err)
		return expectation.ClientRequestError
	}
	if result == nil {
		return nil
	}
	b.logger.Debugf("VK Check result: %v", result)

	if isGroup {
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

	// Ответ в личку
	b.sendMessage(msg.PeerID, utils.FormatVerdict(result), msg.ConversationMessageID)
	return nil
}

func (b *VKBot) handleCommand(msg *object.MessagesMessage) error {
	parts := strings.Fields(msg.Text)
	cmd := strings.ToLower(parts[0])

	var response string
	switch cmd {
	case "/start":
		response = "👋 Привет! Я SpamBreaker для ВКонтакте.\nДобавьте меня в беседу для защиты от спама."
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
	// 1. Проверяем наличие токена в аргументах
	//if len(parts) < 2 {
	//	b.sendMessage(msg.PeerID, "❌ Укажите токен: `/connect ВАШ_ТОКЕН`", msg.ConversationMessageID)
	//	return nil
	//}
	//
	//token := parts[1]
	//chatID := strconv.Itoa(msg.PeerID)
	//
	//// 2. Создаем контекст для запроса к Core API
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()

	//// 3. Вызываем активацию чата
	//if err := b.client.ActivateChat(ctx, token, chatID); err != nil {
	//	b.logger.Errorf("failed to activate chat %s with token: %v", chatID, err)
	//	b.sendMessage(msg.PeerID, "❌ Не удалось подключить бота. Проверьте токен и попробуйте снова.", msg.ConversationMessageID)
	//	return err
	//}
	//
	//// 4. Отправляем успешный ответ
	//successMsg := "✅ *Бот SpamBreaker успешно подключён!*\n\nМодерация сообщений активирована."
	//b.sendMessage(msg.PeerID, successMsg, msg.ConversationMessageID)

	return nil
}
