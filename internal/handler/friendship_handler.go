// Package handler nhận HTTP request và trả response.
package handler

import (
	"errors"
	"net/http"

	"go_service/internal/dto"
	"go_service/internal/middleware"
	"go_service/internal/service"

	"github.com/gin-gonic/gin"
)

// FriendshipHandler xử lý các endpoint tìm kiếm user và kết bạn.
type FriendshipHandler struct {
	friendshipService *service.FriendshipService
}

// NewFriendshipHandler tạo handler, inject FriendshipService.
func NewFriendshipHandler(friendshipService *service.FriendshipService) *FriendshipHandler {
	return &FriendshipHandler{friendshipService: friendshipService}
}

// SearchUser GET /api/v1/friends/search?email=...
func (h *FriendshipHandler) SearchUser(c *gin.Context) {
	callerID := c.GetString(middleware.UserIDKey)

	var req dto.SearchUserRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.friendshipService.SearchUserByEmail(c.Request.Context(), req.Email, callerID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "không tìm thấy người dùng"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// SendFriendRequest POST /api/v1/friends/request
// Gửi lời mời kết bạn (tạo với status=pending).
func (h *FriendshipHandler) SendFriendRequest(c *gin.Context) {
	callerID := c.GetString(middleware.UserIDKey)

	var req dto.SendFriendRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.friendshipService.SendFriendRequest(c.Request.Context(), callerID, req.AddresseeID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFriendshipAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrCannotAddSelf):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "không tìm thấy người dùng"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "đã gửi lời mời kết bạn"})
}

// GetPendingRequests GET /api/v1/friends/requests
// Lấy danh sách lời mời kết bạn đang chờ (caller là addressee).
func (h *FriendshipHandler) GetPendingRequests(c *gin.Context) {
	callerID := c.GetString(middleware.UserIDKey)

	requests, err := h.friendshipService.GetPendingRequests(c.Request.Context(), callerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.PendingRequestListResponse{Requests: requests})
}

// AcceptFriendRequest POST /api/v1/friends/requests/:id/accept
// Chấp nhận lời mời kết bạn.
func (h *FriendshipHandler) AcceptFriendRequest(c *gin.Context) {
	callerID := c.GetString(middleware.UserIDKey)
	friendshipID := c.Param("id")

	err := h.friendshipService.AcceptFriendRequest(c.Request.Context(), friendshipID, callerID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFriendshipNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "không tìm thấy lời mời"})
		case errors.Is(err, service.ErrNotAddressee):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "kết bạn thành công"})
}

// RejectFriendRequest DELETE /api/v1/friends/requests/:id
// Từ chối / xóa lời mời kết bạn.
func (h *FriendshipHandler) RejectFriendRequest(c *gin.Context) {
	callerID := c.GetString(middleware.UserIDKey)
	friendshipID := c.Param("id")

	err := h.friendshipService.RejectFriendRequest(c.Request.Context(), friendshipID, callerID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFriendshipNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "không tìm thấy lời mời"})
		case errors.Is(err, service.ErrNotAddressee):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "đã từ chối lời mời"})
}

// GetFriends GET /api/v1/friends
// Lấy danh sách bạn bè đã kết bạn (accepted).
func (h *FriendshipHandler) GetFriends(c *gin.Context) {
	callerID := c.GetString(middleware.UserIDKey)

	friends, err := h.friendshipService.GetFriends(c.Request.Context(), callerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.FriendListResponse{Friends: friends})
}
