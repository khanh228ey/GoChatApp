package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go_service/internal/dto"
	"go_service/internal/model"
	"go_service/internal/repository"

	"go.mongodb.org/mongo-driver/mongo"
)

var ErrConversationNotFound = errors.New("conversation not found")
var ErrNotParticipant = errors.New("not a conversation participant")

// ConversationService xử lý logic danh sách và trạng thái đọc conversation.
type ConversationService struct {
	convRepo    *repository.ConversationRepository
	messageRepo *repository.MessageRepository
	userRepo    *repository.UserRepository
}

// NewConversationService tạo service, inject repositories.
func NewConversationService(
	convRepo *repository.ConversationRepository,
	messageRepo *repository.MessageRepository,
	userRepo *repository.UserRepository,
) *ConversationService {
	return &ConversationService{
		convRepo:    convRepo,
		messageRepo: messageRepo,
		userRepo:    userRepo,
	}
}

// OnNewMessage cập nhật conversation khi có tin nhắn mới (gọi từ Hub).
func (s *ConversationService) OnNewMessage(ctx context.Context, msg *model.Message, receiverID string) error {
	participantIDs := parseParticipantIDs(msg.ConversationID)
	if len(participantIDs) != 2 {
		return nil
	}

	snapshot := model.LastMessageSnapshot{
		Content:   msg.Content,
		SenderID:  msg.SenderID,
		CreatedAt: msg.CreatedAt,
	}
	return s.convRepo.UpsertOnNewMessage(ctx, msg.ConversationID, participantIDs, snapshot, receiverID)
}

// ListForUser trả danh sách conversation của user, tự backfill từ messages nếu thiếu.
func (s *ConversationService) ListForUser(ctx context.Context, userID string) ([]dto.ConversationResponse, error) {
	if err := s.syncFromMessages(ctx, userID); err != nil {
		return nil, err
	}

	convs, err := s.convRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	peerIDs := make([]string, 0, len(convs))
	for _, conv := range convs {
		if peerID := peerIDFromConversation(conv.ID, userID); peerID != "" {
			peerIDs = append(peerIDs, peerID)
		}
	}

	users, err := s.userRepo.FindByIDs(ctx, peerIDs)
	if err != nil {
		return nil, err
	}
	userMap := make(map[string]model.User, len(users))
	for _, u := range users {
		userMap[u.ID.Hex()] = u
	}

	result := make([]dto.ConversationResponse, 0, len(convs))
	for _, conv := range convs {
		peerID := peerIDFromConversation(conv.ID, userID)
		peer := userMap[peerID]

		unread := 0
		if conv.UnreadCounts != nil {
			unread = conv.UnreadCounts[userID]
		}

		item := dto.ConversationResponse{
			ID:          conv.ID,
			Type:        "direct",
			PeerID:      peerID,
			PeerEmail:   peer.Email,
			UnreadCount: unread,
		}
		if conv.LastMessage.Content != "" {
			item.LastMessage = &struct {
				Content   string `json:"content"`
				SenderID  string `json:"sender_id"`
				CreatedAt string `json:"created_at"`
			}{
				Content:   conv.LastMessage.Content,
				SenderID:  conv.LastMessage.SenderID,
				CreatedAt: conv.LastMessage.CreatedAt.Format(time.RFC3339),
			}
		}
		result = append(result, item)
	}
	return result, nil
}

// MarkAsRead đánh dấu conversation đã đọc cho user.
func (s *ConversationService) MarkAsRead(ctx context.Context, conversationID, userID string) error {
	if !isParticipant(conversationID, userID) {
		return ErrNotParticipant
	}

	_, err := s.convRepo.FindByID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrConversationNotFound
		}
		return err
	}
	return s.convRepo.MarkAsRead(ctx, conversationID, userID)
}

// syncFromMessages backfill conversations từ messages nếu chưa có document.
func (s *ConversationService) syncFromMessages(ctx context.Context, userID string) error {
	msgs, err := s.messageRepo.LatestPerConversation(ctx, userID)
	if err != nil {
		return err
	}

	for _, msg := range msgs {
		participantIDs := parseParticipantIDs(msg.ConversationID)
		if len(participantIDs) != 2 {
			continue
		}

		existing, err := s.convRepo.FindByID(ctx, msg.ConversationID)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}
		if existing != nil {
			continue
		}

		snapshot := model.LastMessageSnapshot{
			Content:   msg.Content,
			SenderID:  msg.SenderID,
			CreatedAt: msg.CreatedAt,
		}
		if err := s.convRepo.UpsertFromSnapshot(ctx, msg.ConversationID, participantIDs, snapshot); err != nil {
			return err
		}
	}
	return nil
}

func parseParticipantIDs(conversationID string) []string {
	parts := strings.Split(conversationID, "_")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil
	}
	return parts
}

func peerIDFromConversation(conversationID, userID string) string {
	for _, id := range parseParticipantIDs(conversationID) {
		if id != userID {
			return id
		}
	}
	return ""
}

func isParticipant(conversationID, userID string) bool {
	for _, id := range parseParticipantIDs(conversationID) {
		if id == userID {
			return true
		}
	}
	return false
}
