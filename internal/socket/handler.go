package socket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	kafkapkg "go_service/internal/kafka"
	"go_service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// wsUpgrader nâng cấp HTTP connection thành WebSocket connection.
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Cho phép mọi origin (dev). Production nên giới hạn domain cụ thể.
	},
}

// Handler xử lý request WebSocket từ client.
type Handler struct {
	hub         *Hub
	authService *service.AuthService // Dùng để xác thực token từ query param
	producer    *kafkapkg.Producer   // Publish tin nhắn lên Kafka thay vì lưu DB trực tiếp
}

// NewHandler tạo handler mới, inject Hub, AuthService và Kafka producer.
func NewHandler(hub *Hub, authService *service.AuthService, producer *kafkapkg.Producer) *Handler {
	return &Handler{hub: hub, authService: authService, producer: producer}
}

// HandleWebSocket là endpoint GET /ws?token=xxx
// Client gửi JWT access token qua query param vì WS không support header tốt.
func (h *Handler) HandleWebSocket(c *gin.Context) {
	// Bước 1: Lấy và xác thực token từ query param
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "thiếu token"})
		return
	}

	userID, err := h.authService.ParseAccessToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token không hợp lệ"})
		return
	}

	// Bước 2: Nâng cấp HTTP → WebSocket
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ws] upgrade failed: %v", err)
		return
	}

	// Bước 3: Tạo Client và đăng ký vào Hub
	client := &Client{
		UserID: userID,
		conn:   conn,
		send:   make(chan []byte, 256),
	}

	h.hub.RegisterClient(client)
	defer func() {
		h.hub.UnregisterClient(client)
		conn.Close()
	}()

	// Bước 4: Chạy WritePump trong goroutine — gửi message từ Hub tới client
	go client.WritePump()

	// Bước 5: Vòng lặp đọc message từ client
	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			return // Client ngắt kết nối
		}
		h.publishIncoming(userID, rawMsg)
	}
}

// publishIncoming validate payload rồi publish lên Kafka (topic chat.messages) thay vì
// lưu DB + route trực tiếp như trước. Việc lưu DB và phát WS được consumer group
// "persist-broadcast" (internal/kafka) đảm nhiệm bất đồng bộ.
func (h *Handler) publishIncoming(senderID string, raw []byte) {
	var payload WsPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("[ws] invalid json from %s: %v", senderID, err)
		return
	}

	if payload.Type != WsTypeChatMessage || payload.ConversationID == "" || payload.Content == "" {
		log.Printf("[ws] invalid payload type=%s convID=%s content=%s", payload.Type, payload.ConversationID, payload.Content)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	evt := kafkapkg.MessageEvent{
		ConversationID: payload.ConversationID,
		SenderID:       senderID,
		Content:        payload.Content,
	}
	if err := h.producer.PublishMessage(ctx, evt); err != nil {
		log.Printf("[ws] failed to publish message to kafka: %v", err)
	}
}
