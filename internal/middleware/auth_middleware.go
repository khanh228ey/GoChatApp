// Package middleware chứa các middleware dùng chung cho toàn bộ HTTP request.
package middleware

import (
	"net/http"
	"strings"

	"go_service/internal/service"

	"github.com/gin-gonic/gin"
)

const UserIDKey = "userID"

// RequireAuth là middleware xác thực JWT Bearer token từ header Authorization.
// Nếu hợp lệ, inject userID vào Gin context để handler dùng tiếp.
func RequireAuth(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "thiếu Authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header không hợp lệ"})
			return
		}

		userID, err := authService.ParseAccessToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token không hợp lệ hoặc đã hết hạn"})
			return
		}

		// Lưu userID vào context để handler dùng
		c.Set(UserIDKey, userID)
		c.Next()
	}
}
