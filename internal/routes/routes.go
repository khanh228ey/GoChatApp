// Package routes đăng ký tất cả HTTP routes của server.
// Mọi endpoint mới nên thêm vào đây thay vì viết trực tiếp trong main.go.
package routes

import (
	"go_service/internal/config"
	"go_service/internal/handler"
	"go_service/internal/middleware"
	"go_service/internal/service"
	"go_service/internal/socket"

	"github.com/gin-gonic/gin"
)

// Setup nhận Gin engine và các handler, đăng ký middleware + routes.
func Setup(
	r *gin.Engine,
	cfg *config.Config,
	socketHandler *socket.Handler,
	authHandler *handler.AuthHandler,
	friendshipHandler *handler.FriendshipHandler,
	authService *service.AuthService,
) {
	// Áp dụng CORS cho mọi request (với credentials cho cookie)
	r.Use(middleware.Cors(cfg))

	// Health check
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// WebSocket endpoint
	r.GET("/ws", socketHandler.HandleWebSocket)

	// API v1
	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.POST("/refresh", authHandler.RefreshToken) // refresh access token bằng cookie
		}

		// Routes yêu cầu xác thực JWT
		protected := v1.Group("")
		protected.Use(middleware.RequireAuth(authService))
		{
			friends := protected.Group("/friends")
			{
				friends.GET("/search", friendshipHandler.SearchUser)              // tìm user theo email
				friends.POST("/request", friendshipHandler.SendFriendRequest)     // gửi lời mời (pending)
				friends.GET("/requests", friendshipHandler.GetPendingRequests)    // inbox lời mời đang chờ
				friends.POST("/requests/:id/accept", friendshipHandler.AcceptFriendRequest) // chấp nhận
				friends.DELETE("/requests/:id", friendshipHandler.RejectFriendRequest)      // từ chối / xóa
				friends.GET("", friendshipHandler.GetFriends)                     // danh sách bạn bè accepted
			}
		}
	}
}
