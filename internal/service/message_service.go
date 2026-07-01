// Package service chứa business logic cho messages.
package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"go_service/internal/dto"
	"go_service/internal/model"
	"go_service/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SaveMessageModel lưu trực tiếp *model.Message đã được build sẵn (dùng bởi Hub).
func (s *MessageService) SaveMessageModel(ctx context.Context, msg *model.Message) error {
	return s.messageRepo.Create(ctx, msg)
}

// MessageService xử lý logic lưu và lấy tin nhắn.
type MessageService struct {
	messageRepo *repository.MessageRepository
}

// NewMessageService tạo service, inject repository.
func NewMessageService(messageRepo *repository.MessageRepository) *MessageService {
	return &MessageService{messageRepo: messageRepo}
}

// BuildConversationID tạo conversation ID deterministic từ 2 userID.
// Luôn sort để A-B và B-A cho cùng 1 ID.
func BuildConversationID(userID1, userID2 string) string {
	ids := []string{userID1, userID2}
	sort.Strings(ids)
	return strings.Join(ids, "_")
}

// SaveMessage lưu message vào DB và trả về message đã có ID.
func (s *MessageService) SaveMessage(ctx context.Context, conversationID, senderID, content string) (*model.Message, error) {
	msg := &model.Message{
		ID:             primitive.NewObjectID(),
		ConversationID: conversationID,
		SenderID:       senderID,
		Content:        content,
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// GetHistory lấy lịch sử tin nhắn của một conversation.
func (s *MessageService) GetHistory(ctx context.Context, conversationID string, limit int64) ([]dto.ChatMessageResponse, error) {
	msgs, err := s.messageRepo.FindByConversationID(ctx, conversationID, limit)
	if err != nil {
		return nil, err
	}

	result := make([]dto.ChatMessageResponse, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, dto.ChatMessageResponse{
			ID:             m.ID.Hex(),
			ConversationID: m.ConversationID,
			SenderID:       m.SenderID,
			Content:        m.Content,
			CreatedAt:      m.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}
