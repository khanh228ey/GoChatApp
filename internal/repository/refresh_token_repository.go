// Package repository xử lý truy vấn MongoDB cho refresh token.
package repository

import (
	"context"
	"time"

	"go_service/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// RefreshTokenRepository thao tác collection "refresh_tokens".
type RefreshTokenRepository struct {
	collection *mongo.Collection
}

// NewRefreshTokenRepository tạo repository với database đã connect.
func NewRefreshTokenRepository(db *mongo.Database) *RefreshTokenRepository {
	return &RefreshTokenRepository{
		collection: db.Collection("refresh_tokens"),
	}
}

// Create lưu refresh token mới vào MongoDB.
func (r *RefreshTokenRepository) Create(ctx context.Context, rt *model.RefreshToken) error {
	rt.CreatedAt = time.Now().UTC()
	_, err := r.collection.InsertOne(ctx, rt)
	return err
}

// FindByToken tìm refresh token theo token string.
func (r *RefreshTokenRepository) FindByToken(ctx context.Context, token string) (*model.RefreshToken, error) {
	var rt model.RefreshToken
	err := r.collection.FindOne(ctx, bson.M{"token": token}).Decode(&rt)
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

// DeleteByToken xóa 1 refresh token — dùng khi rotation.
func (r *RefreshTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"token": token})
	return err
}

// DeleteByUserID xóa tất cả refresh token của 1 user — dùng khi logout.
func (r *RefreshTokenRepository) DeleteByUserID(ctx context.Context, userID primitive.ObjectID) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{"user_id": userID})
	return err
}
