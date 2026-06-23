// Package model chứa struct đại diện document lưu trong MongoDB.
package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RefreshToken là document lưu refresh token trong collection "refresh_tokens".
// Mỗi lần user login, tạo 1 record mới. Khi refresh → rotation: xóa cũ tạo mới.
type RefreshToken struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Token     string             `bson:"token"`      // UUID random string
	ExpiresAt time.Time          `bson:"expires_at"` // Thời điểm hết hạn
	CreatedAt time.Time          `bson:"created_at"`
}
