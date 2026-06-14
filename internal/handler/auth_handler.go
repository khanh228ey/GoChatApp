// Package handler nhận HTTP request và trả response.
package handler

import (
	"errors"
	"net/http"
	"strings"

	"go_service/internal/dto"
	"go_service/internal/service"

	"github.com/gin-gonic/gin"
)

// AuthHandler xử lý các endpoint đăng ký, đăng nhập, đăng xuất.
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

	result, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUserExists) {
			status = http.StatusConflict
		} else if err.Error() == "email hoặc số điện thoại là bắt buộc" {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// Login POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidLogin) {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Logout POST /api/v1/auth/logout — cần gửi header Authorization: Bearer <token>
func (h *AuthHandler) Logout(c *gin.Context) {
	token := extractBearerToken(c.GetHeader("Authorization"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "thiếu token"})
		return
	}

	if err := h.authService.Logout(token); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "đăng xuất thành công"})
}

// extractBearerToken lấy token từ header "Bearer <token>".
func extractBearerToken(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
