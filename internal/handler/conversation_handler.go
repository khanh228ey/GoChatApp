package handler

import (
	"errors"
	"net/http"

	"go_service/internal/dto"
	"go_service/internal/middleware"
	"go_service/internal/service"

	"github.com/gin-gonic/gin"
)

// ConversationHandler xử lý API danh sách và đọc conversation.
type ConversationHandler struct {
	conversationService *service.ConversationService
}

// NewConversationHandler tạo handler, inject service.
func NewConversationHandler(conversationService *service.ConversationService) *ConversationHandler {
	return &ConversationHandler{conversationService: conversationService}
}

// GetConversations GET /api/v1/conversations
func (h *ConversationHandler) GetConversations(c *gin.Context) {
	callerID := c.GetString(middleware.UserIDKey)
	if callerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	convs, err := h.conversationService.ListForUser(c.Request.Context(), callerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "không thể lấy danh sách cuộc trò chuyện"})
		return
	}

	c.JSON(http.StatusOK, dto.ConversationListResponse{Conversations: convs})
}

// MarkConversationRead POST /api/v1/conversations/:id/read
func (h *ConversationHandler) MarkConversationRead(c *gin.Context) {
	callerID := c.GetString(middleware.UserIDKey)
	if callerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "thiếu conversation id"})
		return
	}

	if err := h.conversationService.MarkAsRead(c.Request.Context(), conversationID, callerID); err != nil {
		if errors.Is(err, service.ErrNotParticipant) {
			c.JSON(http.StatusForbidden, gin.H{"error": "không có quyền truy cập"})
			return
		}
		if errors.Is(err, service.ErrConversationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "không tìm thấy cuộc trò chuyện"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "không thể đánh dấu đã đọc"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
