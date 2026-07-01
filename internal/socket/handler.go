package socket

import (
	"log"
	"net/http"

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
}

// NewHandler tạo handler mới, inject Hub và AuthService.
func NewHandler(hub *Hub, authService *service.AuthService) *Handler {
	return &Handler{hub: hub, authService: authService}
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
		h.hub.HandleIncoming(userID, rawMsg)
	}
}
