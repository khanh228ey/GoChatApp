// Package socket xử lý WebSocket: quản lý client, broadcast message realtime.
package socket

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Client đại diện cho 1 kết nối WebSocket đang active.
type Client struct {
	conn *websocket.Conn // Kết nối WebSocket tới browser/app
	send chan []byte     // Channel gửi message tới client (dùng trong WritePump)
}

// Hub là trung tâm quản lý tất cả WebSocket clients.
// Chạy trong goroutine riêng, xử lý register/unregister/broadcast qua channel.
type Hub struct {
	clients    map[*Client]bool // Danh sách client đang online
	register   chan *Client     // Channel nhận client mới kết nối
	unregister chan *Client     // Channel nhận client ngắt kết nối
	broadcast  chan []byte      // Channel nhận message cần gửi cho tất cả client
	mu         sync.RWMutex     // Lock bảo vệ map clients khi đọc/ghi đồng thời
}

// NewHub tạo Hub mới với các channel đã khởi tạo sẵn.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
	}
}

// Run là vòng lặp chính của Hub — lắng nghe và xử lý 3 loại sự kiện.
// Phải chạy trong goroutine: go hub.Run()
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			// Client mới kết nối → thêm vào danh sách
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			// Client ngắt kết nối → xóa khỏi danh sách và đóng channel send
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// Nhận message → gửi tới tất cả client đang online
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client không nhận được → đóng và xóa (tránh block Hub)
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast gửi message tới tất cả client qua channel broadcast.
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// RegisterClient thêm client mới vào Hub.
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient xóa client khỏi Hub khi ngắt kết nối.
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// WritePump đọc message từ channel send và ghi ra WebSocket connection.
// Chạy trong goroutine riêng cho mỗi client.
func (c *Client) WritePump() {
	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
	c.conn.Close()
}
