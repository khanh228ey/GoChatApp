// Package dto chứa struct request/response cho message API.
package dto

// GetMessagesRequest — query params lấy lịch sử tin nhắn.
type GetMessagesRequest struct {
	ConversationID string `form:"conversation_id" binding:"required"`
	Limit          int64  `form:"limit"`
}

// ChatMessageResponse — một tin nhắn trả về client.
type ChatMessageResponse struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	Content        string `json:"content"`
	CreatedAt      string `json:"created_at"`
}

// ChatMessageListResponse — danh sách tin nhắn.
type ChatMessageListResponse struct {
	Messages []ChatMessageResponse `json:"messages"`
}
