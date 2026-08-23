package kafka

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"go_service/internal/model"
	"go_service/internal/service"

	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// wsPayload trùng cấu trúc với socket.WsPayload. Định nghĩa lại ở đây (thay vì import
// package socket) để tránh import cycle: socket → kafka (publish) nên kafka không thể → socket.
type wsPayload struct {
	Type           string `json:"type"`
	ID             string `json:"id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	SenderID       string `json:"sender_id,omitempty"`
	Content        string `json:"content,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
}

// Deliverer là interface tối thiểu Hub cần cung cấp để consumer đẩy message tới WS client.
type Deliverer interface {
	DeliverToUsers(data []byte, userIDs ...string)
}

// PersistBroadcastConsumer đọc topic chat.messages (group "persist-broadcast"): lưu Mongo,
// cập nhật conversation, rồi đẩy qua Hub để phát tới sender + receiver qua WebSocket.
// Đây là consumer group "chính" — nếu nó dừng, tin nhắn vẫn nằm an toàn trong Kafka
// chờ được xử lý tiếp khi consumer chạy lại (không mất dữ liệu).
type PersistBroadcastConsumer struct {
	consumer            *Consumer
	messageService      *service.MessageService
	conversationService *service.ConversationService
	hub                 Deliverer
}

func NewPersistBroadcastConsumer(
	brokers []string,
	topic string,
	messageService *service.MessageService,
	conversationService *service.ConversationService,
	hub Deliverer,
) *PersistBroadcastConsumer {
	return &PersistBroadcastConsumer{
		consumer:            NewConsumer(brokers, topic, "persist-broadcast", "persist-broadcast"),
		messageService:      messageService,
		conversationService: conversationService,
		hub:                 hub,
	}
}

func (p *PersistBroadcastConsumer) Start(ctx context.Context) {
	p.consumer.Start(ctx, p.handle)
}

func (p *PersistBroadcastConsumer) Stats() kafka.ReaderStats { return p.consumer.Stats() }

func (p *PersistBroadcastConsumer) handle(ctx context.Context, m kafka.Message) error {
	var evt MessageEvent
	if err := json.Unmarshal(m.Value, &evt); err != nil {
		return err
	}

	msg := &model.Message{
		ID:             primitive.NewObjectID(),
		ConversationID: evt.ConversationID,
		SenderID:       evt.SenderID,
		Content:        evt.Content,
		CreatedAt:      time.Now().UTC(),
	}

	saveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := p.messageService.SaveMessageModel(saveCtx, msg); err != nil {
		return err
	}
	log.Printf("[kafka:persist-broadcast] message saved: conv=%s sender=%s offset=%d", msg.ConversationID, msg.SenderID, m.Offset)

	receiverID := extractReceiver(evt.ConversationID, evt.SenderID)
	unreadTarget := receiverID
	if unreadTarget == evt.SenderID {
		unreadTarget = ""
	}
	if err := p.conversationService.OnNewMessage(saveCtx, msg, unreadTarget); err != nil {
		log.Printf("[kafka:persist-broadcast] failed to update conversation: %v", err)
	}

	resp := wsPayload{
		Type:           "chat_message",
		ID:             msg.ID.Hex(),
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		Content:        msg.Content,
		CreatedAt:      msg.CreatedAt.Format(time.RFC3339),
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	p.hub.DeliverToUsers(data, evt.SenderID, receiverID)
	return nil
}

// extractReceiver lấy userID còn lại từ conversationID "idA_idB".
// ConversationID được build bằng sort([idA, idB]).join("_").
func extractReceiver(conversationID, senderID string) string {
	idx := strings.Index(conversationID, "_")
	if idx < 0 {
		return ""
	}
	id1 := conversationID[:idx]
	id2 := conversationID[idx+1:]

	if id1 == senderID {
		return id2
	}
	if id2 == senderID {
		return id1
	}
	return id1
}
