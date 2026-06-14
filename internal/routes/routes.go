// Package routes đăng ký tất cả HTTP routes của server.
// Mọi endpoint mới nên thêm vào đây thay vì viết trực tiếp trong main.go.
package routes

import (
	"go_service/internal/handler"
	"go_service/internal/middleware"
	"go_service/internal/socket"

	"github.com/gin-gonic/gin"
)

// Setup nhận Gin engine và các handler, đăng ký middleware + routes.
func Setup(r *gin.Engine, socketHandler *socket.Handler, authHandler *handler.AuthHandler) {
	// Áp dụng CORS cho mọi request
	r.Use(middleware.Cors())

	// Health check — dùng để kiểm tra server còn sống không
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// WebSocket endpoint — client kết nối tại ws://host:port/ws
	r.GET("/ws", socketHandler.HandleWebSocket)

	// API v1
	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
		}
	}
}
