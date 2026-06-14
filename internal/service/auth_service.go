// Package service chứa business logic của ứng dụng.
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go_service/internal/config"
	"go_service/internal/dto"
	"go_service/internal/model"
	"go_service/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists       = errors.New("email hoặc số điện thoại đã được sử dụng")
	ErrInvalidLogin     = errors.New("email/số điện thoại hoặc mật khẩu không đúng")
	ErrTokenInvalid     = errors.New("token không hợp lệ")
	ErrTokenBlacklisted = errors.New("token đã bị vô hiệu hóa")
)

type jwtClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// AuthService xử lý đăng ký, đăng nhập, đăng xuất với JWT.
type AuthService struct {
	userRepo   *repository.UserRepository
	jwtSecret  []byte
	jwtExpire  time.Duration
	blacklist  map[string]time.Time // Token đã logout — lưu tạm trong memory
	blacklistMu sync.RWMutex
}

// NewAuthService tạo service auth, inject repository và config JWT.
func NewAuthService(userRepo *repository.UserRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: []byte(cfg.JWTSecret),
		jwtExpire: time.Duration(cfg.JWTExpireHours) * time.Hour,
		blacklist: make(map[string]time.Time),
	}
}

// Register tạo tài khoản mới — email hoặc sdt + mật khẩu.
func (s *AuthService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.AuthResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if req.Email != "" {
		_, err := s.userRepo.FindByEmail(ctx, req.Email)
		if err == nil {
			return nil, ErrUserExists
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
	}

	if req.Phone != "" {
		_, err := s.userRepo.FindByPhone(ctx, req.Phone)
		if err == nil {
			return nil, ErrUserExists
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		Email:    req.Email,
		Phone:    req.Phone,
		Password: string(hashedPassword),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	token, err := s.generateToken(user.ID.Hex())
	if err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		Token: token,
		User:  toUserResponse(user),
	}, nil
}

// Login xác thực email/sdt + mật khẩu, trả về JWT.
func (s *AuthService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.AuthResponse, error) {
	user, err := s.userRepo.FindByEmailOrPhone(ctx, req.Identifier)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrInvalidLogin
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidLogin
	}

	token, err := s.generateToken(user.ID.Hex())
	if err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		Token: token,
		User:  toUserResponse(user),
	}, nil
}

// Logout vô hiệu hóa token — thêm vào blacklist.
func (s *AuthService) Logout(token string) error {
	claims, err := s.parseToken(token)
	if err != nil {
		return ErrTokenInvalid
	}

	s.blacklistMu.Lock()
	s.blacklist[token] = claims.ExpiresAt.Time
	s.blacklistMu.Unlock()

	return nil
}

// IsTokenBlacklisted kiểm tra token đã logout chưa.
func (s *AuthService) IsTokenBlacklisted(token string) bool {
	s.blacklistMu.RLock()
	defer s.blacklistMu.RUnlock()
	_, exists := s.blacklist[token]
	return exists
}

func (s *AuthService) generateToken(userID string) (string, error) {
	now := time.Now()
	claims := jwtClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtExpire)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) parseToken(tokenString string) (*jwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	if s.IsTokenBlacklisted(tokenString) {
		return nil, ErrTokenBlacklisted
	}

	return claims, nil
}

func toUserResponse(user *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:    user.ID.Hex(),
		Email: user.Email,
		Phone: user.Phone,
	}
}
