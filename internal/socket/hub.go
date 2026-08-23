// Package socket xử lý WebSocket: quản lý client theo userID, phát message realtime.
package socket

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
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
// Lưu ý: Hub không còn tự lưu DB / xử lý business logic khi nhận tin nhắn — việc đó
// nằm ở Kafka consumer (internal/kafka.PersistBroadcastConsumer). Hub chỉ còn là
// tầng transport realtime cuối cùng: giữ kết nối WS và đẩy data khi được yêu cầu.
type Hub struct {
	mu         sync.RWMutex
	clients    map[string]*Client // userID → Client
	register   chan *Client
	unregister chan *Client
}

// NewHub tạo Hub mới.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
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

// DeliverToUsers gửi cùng 1 data tới nhiều user (dedupe id rỗng/trùng). Được gọi bởi
// Kafka consumer (persist-broadcast) sau khi đã lưu DB — không còn gọi trực tiếp từ WS handler.
func (h *Hub) DeliverToUsers(data []byte, userIDs ...string) {
	seen := make(map[string]bool, len(userIDs))
	for _, id := range userIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		h.sendToUser(id, data)
	}
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
