// Package model chứa struct đại diện document lưu trong MongoDB.
package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FriendshipStatus là trạng thái của một friendship request.
type FriendshipStatus string

const (
	FriendshipPending  FriendshipStatus = "pending"
	FriendshipAccepted FriendshipStatus = "accepted"
	FriendshipRejected FriendshipStatus = "rejected"
)

// Friendship là document lưu quan hệ bạn bè trong collection "friendships".
type Friendship struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"    json:"id"`
	RequesterID primitive.ObjectID `bson:"requester_id"     json:"requester_id"`
	AddresseeID primitive.ObjectID `bson:"addressee_id"     json:"addressee_id"`
	Status      FriendshipStatus   `bson:"status"           json:"status"`
	CreatedAt   time.Time          `bson:"created_at"       json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"       json:"updated_at"`
}
