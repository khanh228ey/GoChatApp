// Package socket xử lý WebSocket: quản lý client theo userID, route message realtime.
package socket

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"go_service/internal/model"
	"go_service/internal/service"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WsMessageType là loại event WebSocket.
type WsMessageType string

const (
	WsTypeChatMessage WsMessageType = "chat_message"
	WsTypeError       WsMessageType = "error"
)

// WsPayload là cấu trúc JSON gửi/nhận qua WebSocket.
type WsPayload struct {
	Type           WsMessageType `json:"type"`
	ID             string        `json:"id,omitempty"`
	ConversationID string        `json:"conversation_id,omitempty"`
	SenderID       string        `json:"sender_id,omitempty"`
	Content        string        `json:"content,omitempty"`
	CreatedAt      string        `json:"created_at,omitempty"`
	Error          string        `json:"error,omitempty"`
}

// Client đại diện cho 1 kết nối WebSocket đang active.
type Client struct {
	UserID string
	conn   *websocket.Conn
	send   chan []byte
}

// Hub là trung tâm quản lý tất cả WebSocket clients theo userID.
type Hub struct {
	mu             sync.RWMutex
	clients        map[string]*Client // userID → Client
	register       chan *Client
	unregister     chan *Client
	messageService *service.MessageService
}

// NewHub tạo Hub mới.
func NewHub(messageService *service.MessageService) *Hub {
	return &Hub{
		clients:        make(map[string]*Client),
		register:       make(chan *Client, 16),
		unregister:     make(chan *Client, 16),
		messageService: messageService,
	}
}

// Run là vòng lặp chính của Hub — xử lý register/unregister.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			// Nếu user đã kết nối trước → đóng connection cũ
			if old, ok := h.clients[client.UserID]; ok {
				close(old.send)
			}
			h.clients[client.UserID] = client
			h.mu.Unlock()
			log.Printf("[hub] user connected: %s", client.UserID)

		case client := <-h.unregister:
			h.mu.Lock()
			if cur, ok := h.clients[client.UserID]; ok && cur == client {
				delete(h.clients, client.UserID)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[hub] user disconnected: %s", client.UserID)
		}
	}
}

// sendToUser gửi data tới 1 user cụ thể (thread-safe).
func (h *Hub) sendToUser(userID string, data []byte) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		return // User offline — bỏ qua
	}

	select {
	case client.send <- data:
	default:
		log.Printf("[hub] send buffer full for user: %s", userID)
	}
}

// HandleIncoming xử lý message từ client: lưu DB + route tới receiver.
// Chạy trong goroutine riêng để không block ReadPump.
func (h *Hub) HandleIncoming(senderID string, raw []byte) {
	go func() {
		var payload WsPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			log.Printf("[hub] invalid json from %s: %v", senderID, err)
			return
		}

		if payload.Type != WsTypeChatMessage || payload.ConversationID == "" || payload.Content == "" {
			log.Printf("[hub] invalid payload type=%s convID=%s content=%s", payload.Type, payload.ConversationID, payload.Content)
			return
		}

		// Lưu message vào DB
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		msg := &model.Message{
			ID:             primitive.NewObjectID(),
			ConversationID: payload.ConversationID,
			SenderID:       senderID,
			Content:        payload.Content,
			CreatedAt:      time.Now().UTC(),
		}

		if err := h.messageService.SaveMessageModel(ctx, msg); err != nil {
			log.Printf("[hub] failed to save message: %v", err)
			return
		}
		log.Printf("[hub] message saved: conv=%s sender=%s", msg.ConversationID, senderID)

		// Build response payload (với ID và timestamp từ DB)
		resp := WsPayload{
			Type:           WsTypeChatMessage,
			ID:             msg.ID.Hex(),
			ConversationID: msg.ConversationID,
			SenderID:       senderID,
			Content:        msg.Content,
			CreatedAt:      msg.CreatedAt.Format(time.RFC3339),
		}
		data, _ := json.Marshal(resp)

		// Xác định receiverID từ conversationID (format: "idA_idB" đã sort)
		receiverID := extractReceiver(payload.ConversationID, senderID)
		log.Printf("[hub] routing: sender=%s receiver=%s", senderID, receiverID)

		// Echo lại sender
		h.sendToUser(senderID, data)
		// Gửi tới receiver nếu khác sender và có giá trị
		if receiverID != "" && receiverID != senderID {
			h.sendToUser(receiverID, data)
		}
	}()
}

// extractReceiver lấy userID còn lại từ conversationID "idA_idB".
// ConversationID được build bằng sort([idA, idB]).join("_")
func extractReceiver(conversationID, senderID string) string {
	// Tìm dấu '_' phân cách 2 ID
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
	// senderID không khớp phần nào → trả về id1 để thử
	return id1
}

// RegisterClient thêm client vào Hub.
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient xóa client khỏi Hub.
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// WritePump ghi message từ channel send ra WebSocket connection.
func (c *Client) WritePump() {
	defer c.conn.Close()
	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("[ws] write error user=%s: %v", c.UserID, err)
			break
		}
	}
}
