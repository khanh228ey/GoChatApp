// Package dto chứa struct request/response cho API.
package dto

import "errors"

// RegisterRequest — body đăng ký: email + mật khẩu.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest — body đăng nhập: email + mật khẩu.
type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required"` // email hoặc số điện thoại
	Password   string `json:"password" binding:"required"`
}

// AuthResponse — response sau khi đăng ký/đăng nhập thành công.
// Chỉ trả access_token trong body — refresh_token được set qua HTTP-only Cookie.
type AuthResponse struct {
	AccessToken string       `json:"access_token"`
	User        UserResponse `json:"user"`
}

// UserResponse — thông tin user trả về client (không có password).
type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
}

// MessageResponse — response đơn giản cho logout / refresh.
type MessageResponse struct {
	Message string `json:"message"`
}

// Validate kiểm tra register phải có email.
func (r *RegisterRequest) Validate() error {
	if r.Email == "" {
		return errors.New("email là bắt buộc")
	}
	return nil
}
