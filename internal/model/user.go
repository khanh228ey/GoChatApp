// Package model chứa struct đại diện document lưu trong MongoDB.
package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User là document user trong collection "users".
// Đăng ký bằng email HOẶC số điện thoại + mật khẩu.
type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email     string             `bson:"email,omitempty" json:"email,omitempty"`
	Phone     string             `bson:"phone,omitempty" json:"phone,omitempty"`
	Password  string             `bson:"password" json:"-"` // Không trả về client
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}
