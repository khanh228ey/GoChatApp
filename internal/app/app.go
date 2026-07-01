// Package app gom toàn bộ dependency injection (repo → service → handler).
// Thêm feature mới chỉ cần sửa file này, không làm main.go dài ra.
package app

import (
	"go_service/internal/config"
	"go_service/internal/handler"
	"go_service/internal/repository"
	"go_service/internal/service"
	"go_service/internal/socket"

	"go.mongodb.org/mongo-driver/mongo"
)

// App chứa các dependency đã wire sẵn cho routes và middleware.
type App struct {
	Config            *config.Config
	AuthHandler       *handler.AuthHandler
	FriendshipHandler *handler.FriendshipHandler
	AuthService       *service.AuthService
	SocketHandler     *socket.Handler
}

// New khởi tạo toàn bộ layer từ database đã connect.
func New(cfg *config.Config, db *mongo.Database) *App {
	userRepo := repository.NewUserRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	friendshipRepo := repository.NewFriendshipRepository(db)

	authService := service.NewAuthService(userRepo, refreshTokenRepo, cfg)
	friendshipService := service.NewFriendshipService(userRepo, friendshipRepo)

	hub := socket.NewHub()
	go hub.Run()

	return &App{
		Config:            cfg,
		AuthHandler:       handler.NewAuthHandler(authService),
		FriendshipHandler: handler.NewFriendshipHandler(friendshipService),
		AuthService:       authService,
		SocketHandler:     socket.NewHandler(hub),
	}
}
