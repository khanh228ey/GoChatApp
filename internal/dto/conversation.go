package dto

// ConversationResponse — 1 cuộc trò chuyện trả về cho client.
type ConversationResponse struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // "direct"
	PeerID      string `json:"peer_id"`
	PeerEmail   string `json:"peer_email"`
	UnreadCount int    `json:"unread_count"`
	LastMessage *struct {
		Content   string `json:"content"`
		SenderID  string `json:"sender_id"`
		CreatedAt string `json:"created_at"`
	} `json:"last_message,omitempty"`
}

// ConversationListResponse — danh sách cuộc trò chuyện của user.
type ConversationListResponse struct {
	Conversations []ConversationResponse `json:"conversations"`
}
