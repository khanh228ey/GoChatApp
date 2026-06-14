// Package middleware chứa các middleware dùng chung cho toàn bộ HTTP request.
package middleware

import "github.com/gin-gonic/gin"

// Cors cho phép frontend (chạy domain/port khác) gọi API và WebSocket.
// Trả về 204 cho request OPTIONS (preflight) để browser không chặn request.
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
