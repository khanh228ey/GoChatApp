// Package routes đăng ký tất cả HTTP routes của server.
// Mọi endpoint mới nên thêm vào đây thay vì viết trực tiếp trong main.go.
package routes

import (
	"go_service/internal/app"
	"go_service/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Setup nhận Gin engine và App đã wire sẵn, đăng ký middleware + routes.
func Setup(r *gin.Engine, a *app.App) {
	// Áp dụng CORS cho mọi request (với credentials cho cookie)
	r.Use(middleware.Cors(a.Config))

	// Health check
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// WebSocket endpoint
	r.GET("/ws", a.SocketHandler.HandleWebSocket)

	// API v1
	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", a.AuthHandler.Register)
			auth.POST("/login", a.AuthHandler.Login)
			auth.POST("/logout", a.AuthHandler.Logout)
			auth.POST("/refresh", a.AuthHandler.RefreshToken) // refresh access token bằng cookie
		}

		// Routes yêu cầu xác thực JWT
		protected := v1.Group("")
		protected.Use(middleware.RequireAuth(a.AuthService))
		{
			friends := protected.Group("/friends")
			{
				friends.GET("/search", a.FriendshipHandler.SearchUser)              // tìm user theo email
				friends.POST("/request", a.FriendshipHandler.SendFriendRequest)     // gửi lời mời (pending)
				friends.GET("/requests", a.FriendshipHandler.GetPendingRequests)    // inbox lời mời đang chờ
				friends.POST("/requests/:id/accept", a.FriendshipHandler.AcceptFriendRequest) // chấp nhận
				friends.DELETE("/requests/:id", a.FriendshipHandler.RejectFriendRequest)      // từ chối / xóa
				friends.GET("", a.FriendshipHandler.GetFriends)                     // danh sách bạn bè accepted
			}
		}
	}
}
