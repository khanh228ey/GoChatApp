// File này xử lý kết nối và ngắt kết nối MongoDB.
package config

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDB bọc client và database instance sau khi connect thành công.
// Dùng struct này để truyền DB xuống repository khi cần.
type MongoDB struct {
	Client   *mongo.Client   // Client kết nối tới MongoDB server
	Database *mongo.Database // Database instance để thao tác collection
}

// ConnectMongo tạo kết nối tới MongoDB, ping kiểm tra, rồi trả về MongoDB struct.
func ConnectMongo(cfg *Config) (*MongoDB, error) {
	if cfg.MongoURI == "" {
		return nil, fmt.Errorf("MONGO_URI is required")
	}
	if cfg.MongoDatabase == "" {
		return nil, fmt.Errorf("MONGO_DATABASE is required")
	}

	// Timeout 10 giây để tránh treo khi MongoDB không phản hồi
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}

	// Ping xác nhận kết nối thực sự hoạt động
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongo: %w", err)
	}

	return &MongoDB{
		Client:   client,
		Database: client.Database(cfg.MongoDatabase),
	}, nil
}

// Disconnect đóng kết nối MongoDB khi server shutdown.
func (m *MongoDB) Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.Client.Disconnect(ctx)
}
