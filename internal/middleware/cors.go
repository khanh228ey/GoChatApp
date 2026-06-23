// Package middleware chứa các middleware dùng chung cho toàn bộ HTTP request.
package middleware

import (
	"go_service/internal/config"

	"github.com/gin-gonic/gin"
)

// Cors cho phép frontend gọi API với credentials (cookie).
// QUAN TRỌNG: AllowCredentials=true yêu cầu origin phải cụ thể, không dùng "*".
func Cors(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		// Chỉ allow origin khớp với frontend config
		if origin == cfg.FrontendOrigin {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
