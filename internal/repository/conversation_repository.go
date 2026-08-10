package repository

import (
	"context"
	"time"

	"go_service/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ConversationRepository thao tác collection "conversations".
type ConversationRepository struct {
	col *mongo.Collection
}

// NewConversationRepository tạo repo và đảm bảo index cần thiết.
func NewConversationRepository(db *mongo.Database) *ConversationRepository {
	col := db.Collection("conversations")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "participant_ids", Value: 1}}},
		{Keys: bson.D{{Key: "updated_at", Value: -1}}},
	})

	return &ConversationRepository{col: col}
}

// UpsertOnNewMessage cập nhật/tạo conversation khi có tin nhắn mới.
func (r *ConversationRepository) UpsertOnNewMessage(
	ctx context.Context,
	conversationID string,
	participantIDs []string,
	snapshot model.LastMessageSnapshot,
	receiverID string,
) error {
	now := time.Now().UTC()
	filter := bson.M{"_id": conversationID}
	update := bson.M{
		"$set": bson.M{
			"participant_ids": participantIDs,
			"last_message":    snapshot,
			"updated_at":      now,
		},
	}
	if receiverID != "" {
		update["$inc"] = bson.M{"unread_counts." + receiverID: 1}
	}
	opts := options.Update().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	return err
}

// ListByUserID lấy conversations mà user tham gia, sort mới nhất trước.
func (r *ConversationRepository) ListByUserID(ctx context.Context, userID string) ([]model.Conversation, error) {
	filter := bson.M{"participant_ids": userID}
	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var convs []model.Conversation
	if err := cursor.All(ctx, &convs); err != nil {
		return nil, err
	}
	return convs, nil
}

// FindByID lấy 1 conversation theo ID.
func (r *ConversationRepository) FindByID(ctx context.Context, conversationID string) (*model.Conversation, error) {
	var conv model.Conversation
	err := r.col.FindOne(ctx, bson.M{"_id": conversationID}).Decode(&conv)
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

// MarkAsRead đặt unread count của user về 0.
func (r *ConversationRepository) MarkAsRead(ctx context.Context, conversationID, userID string) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": conversationID},
		bson.M{"$set": bson.M{"unread_counts." + userID: 0}},
	)
	return err
}

// UpsertFromSnapshot tạo/cập nhật conversation từ tin nhắn cuối (dùng backfill).
func (r *ConversationRepository) UpsertFromSnapshot(
	ctx context.Context,
	conversationID string,
	participantIDs []string,
	snapshot model.LastMessageSnapshot,
) error {
	now := time.Now().UTC()
	filter := bson.M{"_id": conversationID}
	update := bson.M{
		"$setOnInsert": bson.M{
			"unread_counts": bson.M{},
		},
		"$set": bson.M{
			"participant_ids": participantIDs,
			"last_message":    snapshot,
			"updated_at":      now,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	return err
}
