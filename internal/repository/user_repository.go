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

// UserRepository thao tác collection "users".
type UserRepository struct {
	collection *mongo.Collection
}

// NewUserRepository tạo repository với database đã connect.
func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{
		collection: db.Collection("users"),
	}
}

// Create lưu user mới vào MongoDB.
func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	result, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		return err
	}

	user.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// FindByEmail tìm user theo email.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID tìm user theo ObjectID.
func (r *UserRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.User, error) {
	var user model.User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByPhone tìm user theo số điện thoại.
func (r *UserRepository) FindByPhone(ctx context.Context, phone string) (*model.User, error) {
	var user model.User
	err := r.collection.FindOne(ctx, bson.M{"phone": phone}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmailOrPhone tìm user theo email hoặc sdt — dùng cho đăng nhập.
func (r *UserRepository) FindByEmailOrPhone(ctx context.Context, identifier string) (*model.User, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"email": identifier},
			{"phone": identifier},
		},
	}

	var user model.User
	err := r.collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
