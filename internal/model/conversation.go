package model

import "time"

// LastMessageSnapshot lưu tin nhắn cuối để hiển thị sidebar.
type LastMessageSnapshot struct {
	Content   string    `bson:"content"    json:"content"`
	SenderID  string    `bson:"sender_id"  json:"sender_id"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

// Conversation là metadata cuộc trò chuyện 1-1, lưu collection "conversations".
// _id = conversation_id dạng "userA_userB" (đã sort).
type Conversation struct {
	ID             string              `bson:"_id"              json:"id"`
	ParticipantIDs []string            `bson:"participant_ids"  json:"participant_ids"`
	LastMessage    LastMessageSnapshot `bson:"last_message"     json:"last_message"`
	UnreadCounts   map[string]int      `bson:"unread_counts"    json:"-"`
	UpdatedAt      time.Time           `bson:"updated_at"       json:"updated_at"`
}
