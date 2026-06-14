package socket

import (
	"log"
	"net/http"

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
	hub *Hub // Hub để register client và broadcast message
}

// NewHandler tạo handler mới, inject Hub vào.
func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

// HandleWebSocket là endpoint GET /ws — xử lý toàn bộ lifecycle của 1 WebSocket connection.
func (h *Handler) HandleWebSocket(c *gin.Context) {
	// Bước 1: Nâng cấp HTTP → WebSocket
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to upgrade websocket"})
		return
	}

	// Bước 2: Tạo Client và đăng ký vào Hub
	client := &Client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.hub.RegisterClient(client)
	defer func() {
		h.hub.UnregisterClient(client)
		conn.Close()
	}()

	// Bước 3: Chạy WritePump trong goroutine — gửi message từ Hub tới client
	go client.WritePump()

	// Bước 4: Vòng lặp đọc message từ client
	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			return // Client ngắt kết nối
		}

		log.Printf("websocket message received: %s", string(rawMsg))
		// Broadcast message tới tất cả client khác (bao gồm cả người gửi)
		h.hub.Broadcast(rawMsg)
	}
}
