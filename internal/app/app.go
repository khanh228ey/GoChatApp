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

	stopConsumers   context.CancelFunc
	persistConsumer *kafkapkg.PersistBroadcastConsumer
	notifyConsumer  *kafkapkg.NotifyConsumer
	accessGate      *kafkapkg.AccessGate
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
	// access.events LUÔN 1 partition — AccessGate cần thứ tự tuyệt đối để làm single-writer coordinator.
	if err := kafkapkg.EnsureTopic(cfg.KafkaBrokers, cfg.KafkaAccessTopic, 1, cfg.KafkaReplicationFactor); err != nil {
		log.Printf("[kafka] không tạo được topic %q (bỏ qua, có thể đã tồn tại hoặc broker chưa sẵn sàng): %v", cfg.KafkaAccessTopic, err)
	}

	producer := kafkapkg.NewProducer(cfg.KafkaBrokers, cfg.KafkaMessagesTopic)

	persistConsumer := kafkapkg.NewPersistBroadcastConsumer(
		cfg.KafkaBrokers, cfg.KafkaMessagesTopic, messageService, conversationService, hub,
	)
	notifyConsumer := kafkapkg.NewNotifyConsumer(cfg.KafkaBrokers, cfg.KafkaMessagesTopic)
	accessGate := kafkapkg.NewAccessGate(cfg.KafkaBrokers, cfg.KafkaAccessTopic)

	// Consumer chạy suốt vòng đời server. access-gate CHỈ được chạy đúng 1 instance
	// (single-writer) — không được scale ra nhiều goroutine/process cùng group này.
	consumerCtx, stopConsumers := context.WithCancel(context.Background())
	go persistConsumer.Start(consumerCtx)
	go notifyConsumer.Start(consumerCtx)
	go accessGate.Start(consumerCtx)

	return &App{
		Config:              cfg,
		AuthHandler:         handler.NewAuthHandler(authService),
		FriendshipHandler:   handler.NewFriendshipHandler(friendshipService),
		MessageHandler:      handler.NewMessageHandler(messageService),
		ConversationHandler: handler.NewConversationHandler(conversationService),
		KafkaHandler:        handler.NewKafkaHandler(producer, persistConsumer, notifyConsumer, accessGate),
		AuthService:         authService,
		SocketHandler:       socket.NewHandler(hub, authService, producer, accessGate),
		KafkaProducer:       producer,

		stopConsumers:   stopConsumers,
		persistConsumer: persistConsumer,
		notifyConsumer:  notifyConsumer,
		accessGate:      accessGate,
	}
}

// Shutdown đóng các Kafka consumer đúng cách (gửi LeaveGroup) trước khi server thoát.
// Nếu bỏ qua bước này, lần khởi động sau (đặc biệt là "access-gate" — chỉ 1 consumer
// cho toàn app) sẽ phải chờ hết SessionTimeout (mặc định 30s) mới join lại được group,
// khiến mọi user bị kẹt ở màn hình chờ trong lúc đó.
func (a *App) Shutdown() {
	a.stopConsumers()
	if err := a.persistConsumer.Close(); err != nil {
		log.Printf("[kafka] close persist-broadcast consumer error: %v", err)
	}
	if err := a.notifyConsumer.Close(); err != nil {
		log.Printf("[kafka] close notify-unread consumer error: %v", err)
	}
	if err := a.accessGate.Close(); err != nil {
		log.Printf("[kafka] close access-gate consumer error: %v", err)
	}
}
