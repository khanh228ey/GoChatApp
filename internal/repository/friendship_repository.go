// Package repository xử lý truy vấn MongoDB.
package repository

import (
	"context"
	"time"

	"go_service/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// FriendshipRepository thao tác collection "friendships".
type FriendshipRepository struct {
	collection *mongo.Collection
}

// NewFriendshipRepository tạo repository với database đã connect.
func NewFriendshipRepository(db *mongo.Database) *FriendshipRepository {
	return &FriendshipRepository{
		collection: db.Collection("friendships"),
	}
}

// Create tạo một friendship request mới.
func (r *FriendshipRepository) Create(ctx context.Context, f *model.Friendship) error {
	now := time.Now().UTC()
	f.CreatedAt = now
	f.UpdatedAt = now

	result, err := r.collection.InsertOne(ctx, f)
	if err != nil {
		return err
	}
	f.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// FindByID tìm friendship theo _id.
func (r *FriendshipRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.Friendship, error) {
	var f model.Friendship
	if err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

// FindBetween tìm friendship (dù hướng nào) giữa 2 user.
func (r *FriendshipRepository) FindBetween(ctx context.Context, userA, userB primitive.ObjectID) (*model.Friendship, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"requester_id": userA, "addressee_id": userB},
			{"requester_id": userB, "addressee_id": userA},
		},
	}
	var f model.Friendship
	if err := r.collection.FindOne(ctx, filter).Decode(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

// FindAcceptedByUserID lấy tất cả friendship có status "accepted" của một user.
func (r *FriendshipRepository) FindAcceptedByUserID(ctx context.Context, userID primitive.ObjectID) ([]model.Friendship, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"requester_id": userID},
			{"addressee_id": userID},
		},
		"status": model.FriendshipAccepted,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var friendships []model.Friendship
	if err := cursor.All(ctx, &friendships); err != nil {
		return nil, err
	}
	return friendships, nil
}

// FindPendingByAddresseeID lấy các lời mời kết bạn đang chờ mà user là người nhận.
func (r *FriendshipRepository) FindPendingByAddresseeID(ctx context.Context, addresseeID primitive.ObjectID) ([]model.Friendship, error) {
	filter := bson.M{
		"addressee_id": addresseeID,
		"status":       model.FriendshipPending,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var friendships []model.Friendship
	if err := cursor.All(ctx, &friendships); err != nil {
		return nil, err
	}
	return friendships, nil
}

// UpdateStatus cập nhật trạng thái friendship theo ID.
func (r *FriendshipRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status model.FriendshipStatus) error {
	_, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"status": status, "updated_at": time.Now().UTC()}},
	)
	return err
}

// DeleteByID xóa một friendship theo _id.
func (r *FriendshipRepository) DeleteByID(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
