// Package repository xử lý CRUD với MongoDB cho messages.
package repository

import (
	"context"
	"time"

	"go_service/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MessageRepository thao tác với collection "messages".
type MessageRepository struct {
	col *mongo.Collection
}

// NewMessageRepository tạo repo, tự đảm bảo index compound (conversation_id + created_at).
func NewMessageRepository(db *mongo.Database) *MessageRepository {
	col := db.Collection("messages")

	// Index để query theo conversation_id và sort theo created_at nhanh
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "conversation_id", Value: 1},
			{Key: "created_at", Value: 1},
		},
	})

	return &MessageRepository{col: col}
}

// Create lưu 1 message mới vào DB.
func (r *MessageRepository) Create(ctx context.Context, msg *model.Message) error {
	_, err := r.col.InsertOne(ctx, msg)
	return err
}

// FindByConversationID lấy tối đa `limit` messages mới nhất của 1 conversation.
// Trả về theo thứ tự cũ → mới (asc created_at).
func (r *MessageRepository) FindByConversationID(ctx context.Context, conversationID string, limit int64) ([]model.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Lấy `limit` messages MỚI NHẤT → sort desc, lấy limit, rồi reverse
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit)

	cursor, err := r.col.Find(ctx, bson.M{"conversation_id": conversationID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var msgs []model.Message
	if err := cursor.All(ctx, &msgs); err != nil {
		return nil, err
	}

	// Reverse để trả về thứ tự cũ → mới
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nil
}
