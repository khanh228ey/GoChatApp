// Package app gom toàn bộ dependency injection (repo → service → handler).
// Thêm feature mới chỉ cần sửa file này, không làm main.go dài ra.
package app

import (
	"context"
	"log"

	"go_service/internal/config"
	"go_service/internal/handler"
	kafkapkg "go_service/internal/kafka"
	"go_service/internal/repository"
	"go_service/internal/service"
	"go_service/internal/socket"

	"go.mongodb.org/mongo-driver/mongo"
)

// App chứa các dependency đã wire sẵn cho routes và middleware.
type App struct {
	Config              *config.Config
	AuthHandler         *handler.AuthHandler
	FriendshipHandler   *handler.FriendshipHandler
	MessageHandler      *handler.MessageHandler
	ConversationHandler *handler.ConversationHandler
	KafkaHandler        *handler.KafkaHandler
	AuthService         *service.AuthService
	SocketHandler       *socket.Handler
	KafkaProducer       *kafkapkg.Producer
}

// New khởi tạo toàn bộ layer từ database đã connect.
func New(cfg *config.Config, db *mongo.Database) *App {
	userRepo := repository.NewUserRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	friendshipRepo := repository.NewFriendshipRepository(db)
	messageRepo := repository.NewMessageRepository(db)
	conversationRepo := repository.NewConversationRepository(db)

	authService := service.NewAuthService(userRepo, refreshTokenRepo, cfg)
	friendshipService := service.NewFriendshipService(userRepo, friendshipRepo)
	messageService := service.NewMessageService(messageRepo)
	conversationService := service.NewConversationService(conversationRepo, messageRepo, userRepo)

	hub := socket.NewHub()
	go hub.Run()

	// Tạo topic nếu chưa có (idempotent). Không Fatal nếu lỗi — broker có thể chưa sẵn
	// sàng lúc server khởi động, hoặc topic đã được tạo sẵn từ trước.
	if err := kafkapkg.EnsureTopic(cfg.KafkaBrokers, cfg.KafkaMessagesTopic, cfg.KafkaMessagesPartitions, cfg.KafkaReplicationFactor); err != nil {
		log.Printf("[kafka] không tạo được topic %q (bỏ qua, có thể đã tồn tại hoặc broker chưa sẵn sàng): %v", cfg.KafkaMessagesTopic, err)
	}

	producer := kafkapkg.NewProducer(cfg.KafkaBrokers, cfg.KafkaMessagesTopic)

	persistConsumer := kafkapkg.NewPersistBroadcastConsumer(
		cfg.KafkaBrokers, cfg.KafkaMessagesTopic, messageService, conversationService, hub,
	)
	notifyConsumer := kafkapkg.NewNotifyConsumer(cfg.KafkaBrokers, cfg.KafkaMessagesTopic)

	// 2 consumer group độc lập cùng đọc 1 topic — chạy suốt vòng đời server.
	consumerCtx := context.Background()
	go persistConsumer.Start(consumerCtx)
	go notifyConsumer.Start(consumerCtx)

	return &App{
		Config:              cfg,
		AuthHandler:         handler.NewAuthHandler(authService),
		FriendshipHandler:   handler.NewFriendshipHandler(friendshipService),
		MessageHandler:      handler.NewMessageHandler(messageService),
		ConversationHandler: handler.NewConversationHandler(conversationService),
		KafkaHandler:        handler.NewKafkaHandler(producer, persistConsumer, notifyConsumer),
		AuthService:         authService,
		SocketHandler:       socket.NewHandler(hub, authService, producer),
		KafkaProducer:       producer,
	}
}
