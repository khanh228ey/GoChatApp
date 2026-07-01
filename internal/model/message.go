// Package model chứa struct đại diện document lưu trong MongoDB.
package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Message là document lưu tin nhắn trong collection "messages".
type Message struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ConversationID string             `bson:"conversation_id"  json:"conversation_id"` // e.g. "userA_userB" (sorted)
	SenderID       string             `bson:"sender_id"        json:"sender_id"`
	Content        string             `bson:"content"          json:"content"`
	CreatedAt      time.Time          `bson:"created_at"       json:"created_at"`
}
