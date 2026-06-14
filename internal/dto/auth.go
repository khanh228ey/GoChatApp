// Package dto chứa struct request/response cho API.
package dto

import "errors"

// RegisterRequest — body đăng ký: email hoặc sdt + mật khẩu.
type RegisterRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest — body đăng nhập: nhập email hoặc sdt vào 1 field + mật khẩu.
type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required"` // email hoặc số điện thoại
	Password   string `json:"password" binding:"required"`
}

// AuthResponse — response sau khi đăng ký/đăng nhập thành công.
type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// UserResponse — thông tin user trả về client (không có password).
type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// MessageResponse — response đơn giản cho logout.
type MessageResponse struct {
	Message string `json:"message"`
}

// Validate kiểm tra register phải có ít nhất email hoặc sdt.
func (r *RegisterRequest) Validate() error {
	if r.Email == "" && r.Phone == "" {
		return errors.New("email hoặc số điện thoại là bắt buộc")
	}
	return nil
}
