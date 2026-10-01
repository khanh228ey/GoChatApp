// Package main là điểm khởi động (entry point) của toàn bộ server.
// File này chỉ làm nhiệm vụ: load config → connect DB → chạy HTTP server.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go_service/internal/app"
	"go_service/internal/config"
	"go_service/internal/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	mongo, err := config.ConnectMongo(cfg)
	if err != nil {
		log.Fatalf("failed to connect mongodb: %v", err)
	}
	log.Printf("connected to mongodb database: %s", cfg.MongoDatabase)

	application := app.New(cfg, mongo.Database)

	r := gin.Default()
	routes.Setup(r, application)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	go func() {
		log.Printf("server running on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	// Chờ Ctrl+C / SIGTERM rồi dọn dẹp đúng cách — quan trọng nhất là đóng các Kafka
	// consumer (gửi LeaveGroup) để lần chạy `go run` tiếp theo không bị kẹt chờ hết
	// SessionTimeout (mặc định 30s) mới join lại được group "access-gate".
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http server shutdown error: %v", err)
	}

	application.Shutdown()

	if err := application.KafkaProducer.Close(); err != nil {
		log.Printf("failed to close kafka producer: %v", err)
	}
	if err := mongo.Disconnect(); err != nil {
		log.Printf("failed to disconnect mongodb: %v", err)
	}
}
