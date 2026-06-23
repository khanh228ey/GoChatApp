// Package service chứa business logic của ứng dụng.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	ErrUserExists          = errors.New("email đã được sử dụng")
	ErrInvalidLogin        = errors.New("email hoặc mật khẩu không đúng")
	ErrTokenInvalid        = errors.New("token không hợp lệ")
	ErrRefreshTokenExpired = errors.New("refresh token đã hết hạn, vui lòng đăng nhập lại")
)

type jwtClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// TokenPair chứa cả access token lẫn refresh token — dùng nội bộ giữa service và handler.
type TokenPair struct {
	AccessToken        string
	RefreshToken       string
	RefreshTokenExpiry time.Time
}

// AuthService xử lý đăng ký, đăng nhập, đăng xuất với JWT + Refresh Token.
type AuthService struct {
	userRepo            *repository.UserRepository
	refreshTokenRepo    *repository.RefreshTokenRepository
	jwtSecret           []byte
	accessTokenExpire   time.Duration
	refreshTokenExpire  time.Duration
}

// NewAuthService tạo service auth, inject repository và config.
func NewAuthService(
	userRepo *repository.UserRepository,
	refreshTokenRepo *repository.RefreshTokenRepository,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		userRepo:           userRepo,
		refreshTokenRepo:   refreshTokenRepo,
		jwtSecret:          []byte(cfg.JWTSecret),
		accessTokenExpire:  time.Duration(cfg.AccessTokenExpireMinutes) * time.Minute,
		refreshTokenExpire: time.Duration(cfg.RefreshTokenExpireDays) * 24 * time.Hour,
	}
}

// Register tạo tài khoản mới — email + mật khẩu.
func (s *AuthService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.AuthResponse, *TokenPair, error) {
	if err := req.Validate(); err != nil {
		return nil, nil, err
	}

	// Kiểm tra email đã tồn tại chưa
	_, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err == nil {
		return nil, nil, ErrUserExists
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		Email:    req.Email,
		Password: string(hashedPassword),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, nil, err
	}

	pair, err := s.generateTokenPair(ctx, user.ID.Hex())
	if err != nil {
		return nil, nil, err
	}

	return &dto.AuthResponse{
		AccessToken: pair.AccessToken,
		User:        toUserResponse(user),
	}, pair, nil
}

// Login xác thực email + mật khẩu, trả về token pair.
func (s *AuthService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.AuthResponse, *TokenPair, error) {
	user, err := s.userRepo.FindByEmailOrPhone(ctx, req.Identifier)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil, ErrInvalidLogin
		}
		return nil, nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, nil, ErrInvalidLogin
	}

	pair, err := s.generateTokenPair(ctx, user.ID.Hex())
	if err != nil {
		return nil, nil, err
	}

	return &dto.AuthResponse{
		AccessToken: pair.AccessToken,
		User:        toUserResponse(user),
	}, pair, nil
}

// RefreshToken validate refresh token từ cookie, rotation, trả về token pair mới.
func (s *AuthService) RefreshToken(ctx context.Context, oldRefreshToken string) (*dto.AuthResponse, *TokenPair, error) {
	// Tìm refresh token trong DB
	rt, err := s.refreshTokenRepo.FindByToken(ctx, oldRefreshToken)
	if err != nil {
		return nil, nil, ErrTokenInvalid
	}

	// Kiểm tra hết hạn
	if time.Now().After(rt.ExpiresAt) {
		// Xóa token hết hạn
		_ = s.refreshTokenRepo.DeleteByToken(ctx, oldRefreshToken)
		return nil, nil, ErrRefreshTokenExpired
	}

	// Tìm user
	user, err := s.userRepo.FindByID(ctx, rt.UserID)
	if err != nil {
		return nil, nil, ErrTokenInvalid
	}

	// Rotation: xóa token cũ
	if err := s.refreshTokenRepo.DeleteByToken(ctx, oldRefreshToken); err != nil {
		return nil, nil, fmt.Errorf("delete old refresh token: %w", err)
	}

	// Tạo token pair mới
	pair, err := s.generateTokenPair(ctx, user.ID.Hex())
	if err != nil {
		return nil, nil, err
	}

	return &dto.AuthResponse{
		AccessToken: pair.AccessToken,
		User:        toUserResponse(user),
	}, pair, nil
}

// Logout xóa tất cả refresh token của user khỏi DB.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	rt, err := s.refreshTokenRepo.FindByToken(ctx, refreshToken)
	if err != nil {
		// Token không tồn tại — vẫn coi là logout thành công
		return nil
	}
	return s.refreshTokenRepo.DeleteByUserID(ctx, rt.UserID)
}

// generateTokenPair tạo access token (JWT) + refresh token (UUID random), lưu refresh token vào DB.
func (s *AuthService) generateTokenPair(ctx context.Context, userID string) (*TokenPair, error) {
	accessToken, err := s.generateAccessToken(userID)
	if err != nil {
		return nil, err
	}

	refreshToken, expiresAt, err := s.generateRefreshToken(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:        accessToken,
		RefreshToken:       refreshToken,
		RefreshTokenExpiry: expiresAt,
	}, nil
}

// generateAccessToken tạo JWT access token với thời hạn ngắn.
func (s *AuthService) generateAccessToken(userID string) (string, error) {
	now := time.Now()
	claims := jwtClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTokenExpire)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// generateRefreshToken tạo UUID random string, lưu vào DB, trả về token string + thời hạn.
func (s *AuthService) generateRefreshToken(ctx context.Context, userID string) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, fmt.Errorf("generate refresh token: %w", err)
	}
	tokenString := hex.EncodeToString(b)

	userOID, err := parseObjectID(userID)
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := time.Now().UTC().Add(s.refreshTokenExpire)
	rt := &model.RefreshToken{
		UserID:    userOID,
		Token:     tokenString,
		ExpiresAt: expiresAt,
	}

	if err := s.refreshTokenRepo.Create(ctx, rt); err != nil {
		return "", time.Time{}, fmt.Errorf("save refresh token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ParseAccessToken validate và parse JWT access token — dùng bởi middleware.
func (s *AuthService) ParseAccessToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", ErrTokenInvalid
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return "", ErrTokenInvalid
	}

	return claims.UserID, nil
}

func toUserResponse(user *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:    user.ID.Hex(),
		Email: user.Email,
	}
}
