// Package handler xử lý HTTP request cho message API.
package handler

import (
	"net/http"

	"go_service/internal/dto"
	"go_service/internal/middleware"
	"go_service/internal/service"

	"github.com/gin-gonic/gin"
)

// MessageHandler xử lý các request liên quan đến messages.
type MessageHandler struct {
	messageService *service.MessageService
}

// NewMessageHandler tạo handler, inject service.
func NewMessageHandler(messageService *service.MessageService) *MessageHandler {
	return &MessageHandler{messageService: messageService}
}

// GetMessages lấy lịch sử tin nhắn của một conversation.
// GET /api/v1/messages?conversation_id=xxx&limit=50
func (h *MessageHandler) GetMessages(c *gin.Context) {
	var req dto.GetMessagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "thiếu conversation_id"})
		return
	}

	// Chỉ cho phép lấy message của conversation mà caller tham gia
	callerID := c.GetString(middleware.UserIDKey)
	if callerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	messages, err := h.messageService.GetHistory(c.Request.Context(), req.ConversationID, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "không thể lấy tin nhắn"})
		return
	}

	c.JSON(http.StatusOK, dto.ChatMessageListResponse{Messages: messages})
}
