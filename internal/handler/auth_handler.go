// Package handler nhận HTTP request và trả response.
package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"go_service/internal/dto"
	"go_service/internal/service"

	"github.com/gin-gonic/gin"
)

const refreshTokenCookieName = "refresh_token"

// AuthHandler xử lý các endpoint đăng ký, đăng nhập, đăng xuất, refresh token.
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler tạo handler, inject AuthService.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, pair, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUserExists) {
			status = http.StatusConflict
		} else if err.Error() == "email là bắt buộc" {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	setRefreshTokenCookie(c, pair.RefreshToken, pair.RefreshTokenExpiry)
	c.JSON(http.StatusCreated, result)
}

// Login POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, pair, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidLogin) {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	setRefreshTokenCookie(c, pair.RefreshToken, pair.RefreshTokenExpiry)
	c.JSON(http.StatusOK, result)
}

// RefreshToken POST /api/v1/auth/refresh
// Browser tự gửi HTTP-only cookie, không cần body.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	oldToken, err := c.Cookie(refreshTokenCookieName)
	if err != nil || oldToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token không tìm thấy"})
		return
	}

	result, pair, err := h.authService.RefreshToken(c.Request.Context(), oldToken)
	if err != nil {
		// Xóa cookie nếu token không hợp lệ / hết hạn
		clearRefreshTokenCookie(c)
		status := http.StatusUnauthorized
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	setRefreshTokenCookie(c, pair.RefreshToken, pair.RefreshTokenExpiry)
	c.JSON(http.StatusOK, result)
}

// Logout POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie(refreshTokenCookieName)

	if err := h.authService.Logout(c.Request.Context(), refreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	clearRefreshTokenCookie(c)
	c.JSON(http.StatusOK, dto.MessageResponse{Message: "đăng xuất thành công"})
}

// setRefreshTokenCookie set HTTP-only cookie chứa refresh token.
func setRefreshTokenCookie(c *gin.Context, token string, expiry time.Time) {
	maxAge := int(time.Until(expiry).Seconds())
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshTokenCookieName, token, maxAge, "/api/v1/auth", "", false, true)
}

// clearRefreshTokenCookie xóa cookie refresh token.
func clearRefreshTokenCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshTokenCookieName, "", -1, "/api/v1/auth", "", false, true)
}

// extractBearerToken lấy token từ header "Bearer <token>".
func extractBearerToken(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
