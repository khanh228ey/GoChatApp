// Package service chứa business logic của ứng dụng.
package service

import (
	"context"
	"errors"
	"fmt"

	"go_service/internal/dto"
	"go_service/internal/model"
	"go_service/internal/repository"

	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrFriendshipAlreadyExists = errors.New("đã có lời mời kết bạn hoặc đã là bạn bè")
	ErrCannotAddSelf           = errors.New("không thể kết bạn với chính mình")
	ErrUserNotFound            = errors.New("không tìm thấy người dùng")
	ErrFriendshipNotFound      = errors.New("không tìm thấy lời mời kết bạn")
	ErrNotAddressee            = errors.New("bạn không phải người nhận lời mời này")
)

// FriendshipService xử lý logic tìm kiếm user và kết bạn.
type FriendshipService struct {
	userRepo       *repository.UserRepository
	friendshipRepo *repository.FriendshipRepository
}

// NewFriendshipService tạo service, inject repositories.
func NewFriendshipService(userRepo *repository.UserRepository, friendshipRepo *repository.FriendshipRepository) *FriendshipService {
	return &FriendshipService{
		userRepo:       userRepo,
		friendshipRepo: friendshipRepo,
	}
}

// SearchUserByEmail tìm user theo email, không trả về bản thân.
func (s *FriendshipService) SearchUserByEmail(ctx context.Context, email string, callerID string) (*dto.SearchUserResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Không trả về chính mình
	if user.ID.Hex() == callerID {
		return nil, ErrUserNotFound
	}

	return &dto.SearchUserResponse{
		ID:    user.ID.Hex(),
		Email: user.Email,
	}, nil
}

// SendFriendRequest gửi lời mời kết bạn (tạo với status = pending).
func (s *FriendshipService) SendFriendRequest(ctx context.Context, requesterIDStr, addresseeIDStr string) error {
	if requesterIDStr == addresseeIDStr {
		return ErrCannotAddSelf
	}

	requesterOID, err := parseObjectID(requesterIDStr)
	if err != nil {
		return err
	}
	addresseeOID, err := parseObjectID(addresseeIDStr)
	if err != nil {
		return err
	}

	// Kiểm tra đã tồn tại friendship chưa (pending hoặc accepted)
	existing, err := s.friendshipRepo.FindBetween(ctx, requesterOID, addresseeOID)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	if existing != nil {
		return ErrFriendshipAlreadyExists
	}

	// Kiểm tra addressee có tồn tại không
	_, err = s.userRepo.FindByID(ctx, addresseeOID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrUserNotFound
		}
		return err
	}

	friendship := &model.Friendship{
		RequesterID: requesterOID,
		AddresseeID: addresseeOID,
		Status:      model.FriendshipPending, // Chờ người nhận xác nhận
	}

	return s.friendshipRepo.Create(ctx, friendship)
}

// GetPendingRequests lấy danh sách lời mời kết bạn đang chờ mà user là người nhận.
func (s *FriendshipService) GetPendingRequests(ctx context.Context, userIDStr string) ([]dto.PendingRequestItem, error) {
	userOID, err := parseObjectID(userIDStr)
	if err != nil {
		return nil, err
	}

	friendships, err := s.friendshipRepo.FindPendingByAddresseeID(ctx, userOID)
	if err != nil {
		return nil, err
	}

	items := make([]dto.PendingRequestItem, 0, len(friendships))
	for _, f := range friendships {
		requester, err := s.userRepo.FindByID(ctx, f.RequesterID)
		if err != nil {
			continue // bỏ qua user đã bị xóa
		}
		items = append(items, dto.PendingRequestItem{
			FriendshipID: f.ID.Hex(),
			RequesterID:  f.RequesterID.Hex(),
			Email:        requester.Email,
		})
	}
	return items, nil
}

// AcceptFriendRequest chấp nhận một lời mời kết bạn — chỉ addressee mới được accept.
func (s *FriendshipService) AcceptFriendRequest(ctx context.Context, friendshipIDStr, callerIDStr string) error {
	friendshipOID, err := parseObjectID(friendshipIDStr)
	if err != nil {
		return err
	}

	f, err := s.friendshipRepo.FindByID(ctx, friendshipOID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrFriendshipNotFound
		}
		return err
	}

	// Chỉ addressee mới được chấp nhận
	if f.AddresseeID.Hex() != callerIDStr {
		return ErrNotAddressee
	}

	if f.Status != model.FriendshipPending {
		return fmt.Errorf("lời mời không còn ở trạng thái chờ")
	}

	return s.friendshipRepo.UpdateStatus(ctx, friendshipOID, model.FriendshipAccepted)
}

// RejectFriendRequest từ chối / xóa lời mời kết bạn — chỉ addressee mới được reject.
func (s *FriendshipService) RejectFriendRequest(ctx context.Context, friendshipIDStr, callerIDStr string) error {
	friendshipOID, err := parseObjectID(friendshipIDStr)
	if err != nil {
		return err
	}

	f, err := s.friendshipRepo.FindByID(ctx, friendshipOID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrFriendshipNotFound
		}
		return err
	}

	// Chỉ addressee mới được từ chối
	if f.AddresseeID.Hex() != callerIDStr {
		return ErrNotAddressee
	}

	return s.friendshipRepo.DeleteByID(ctx, friendshipOID)
}

// GetFriends lấy danh sách bạn bè đã accepted của một user.
func (s *FriendshipService) GetFriends(ctx context.Context, userIDStr string) ([]dto.FriendResponse, error) {
	userOID, err := parseObjectID(userIDStr)
	if err != nil {
		return nil, err
	}

	friendships, err := s.friendshipRepo.FindAcceptedByUserID(ctx, userOID)
	if err != nil {
		return nil, err
	}

	friends := make([]dto.FriendResponse, 0, len(friendships))
	for _, f := range friendships {
		// Lấy ID của người bạn (không phải chính mình)
		friendOID := f.RequesterID
		if f.RequesterID == userOID {
			friendOID = f.AddresseeID
		}

		friend, err := s.userRepo.FindByID(ctx, friendOID)
		if err != nil {
			continue // bỏ qua user đã bị xóa
		}

		friends = append(friends, dto.FriendResponse{
			ID:    friend.ID.Hex(),
			Email: friend.Email,
		})
	}

	return friends, nil
}
