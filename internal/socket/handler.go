package socket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
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
	accessGate  *kafkapkg.AccessGate // Giới hạn số user online cùng lúc (xem internal/kafka/access_gate.go)
}

// NewHandler tạo handler mới, inject Hub, AuthService, Kafka producer và AccessGate.
func NewHandler(hub *Hub, authService *service.AuthService, producer *kafkapkg.Producer, accessGate *kafkapkg.AccessGate) *Handler {
	return &Handler{hub: hub, authService: authService, producer: producer, accessGate: accessGate}
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

	// Bước 3: Tạo Client, chạy WritePump ngay (an toàn dù chưa được đăng ký vào Hub) —
	// việc "vào Hub" (được coi là online, nhận/gửi chat) chỉ xảy ra SAU KHI AccessGate cấp slot.
	client := &Client{
		UserID: userID,
		conn:   conn,
		send:   make(chan []byte, 256),
	}
	go client.WritePump()

	// Bước 4: Xin 1 slot từ AccessGate (đi qua Kafka). Nếu hết chỗ, client nhận "waiting"
	// và chỉ vào Hub khi có người khác rời đi giải phóng slot.
	var granted atomic.Bool
	connCtx, cancelConn := context.WithCancel(context.Background())

	h.sendStatus(client, WsTypeWaiting)
	waitCh := h.accessGate.RequestAccess(connCtx, userID)
	go func() {
		select {
		case <-waitCh:
			if granted.CompareAndSwap(false, true) {
				h.hub.RegisterClient(client)
				h.sendStatus(client, WsTypeAccessGranted)
			}
		case <-connCtx.Done():
			// Connection đã đóng trước khi kịp được cấp slot — không làm gì thêm.
		}
	}()

	defer func() {
		cancelConn() // đánh thức goroutine chờ ở trên nếu còn đang chờ, tránh leak

		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		h.accessGate.Release(releaseCtx, userID)
		cancel()

		if granted.Load() {
			h.hub.UnregisterClient(client) // đóng send channel qua Hub
		} else {
			close(client.send) // chưa từng được Hub quản lý → tự đóng để WritePump kết thúc
		}
		conn.Close()
	}()

	// Bước 5: Vòng lặp đọc message từ client — luôn đọc để phát hiện disconnect ngay cả
	// khi đang chờ, nhưng bỏ qua nội dung gửi lên trước khi được cấp slot.
	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			return // Client ngắt kết nối
		}
		if !granted.Load() {
			continue
		}
		h.publishIncoming(userID, rawMsg)
	}
}

// sendStatus gửi 1 WsPayload chỉ có Type (waiting/access_granted...) tới client.
func (h *Handler) sendStatus(client *Client, t WsMessageType) {
	data, _ := json.Marshal(WsPayload{Type: t})
	select {
	case client.send <- data:
	default:
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
