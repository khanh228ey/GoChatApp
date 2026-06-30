// Package dto chứa struct request/response cho friendship API.
package dto

// SearchUserRequest — query param tìm kiếm user theo email.
type SearchUserRequest struct {
	Email string `form:"email" binding:"required,email"`
}

// SearchUserResponse — thông tin user trả về khi tìm kiếm.
type SearchUserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// SendFriendRequestRequest — body gửi lời mời kết bạn.
type SendFriendRequestRequest struct {
	AddresseeID string `json:"addressee_id" binding:"required"`
}

// FriendResponse — thông tin 1 người bạn đã accepted.
type FriendResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// FriendListResponse — danh sách bạn bè đã accepted.
type FriendListResponse struct {
	Friends []FriendResponse `json:"friends"`
}

// PendingRequestItem — một lời mời kết bạn đang chờ (từ phía người nhận nhìn).
type PendingRequestItem struct {
	FriendshipID string `json:"friendship_id"` // _id của document friendship
	RequesterID  string `json:"requester_id"`
	Email        string `json:"email"`   // email người gửi lời mời
}

// PendingRequestListResponse — danh sách lời mời đang chờ.
type PendingRequestListResponse struct {
	Requests []PendingRequestItem `json:"requests"`
}

// AcceptRejectRequest — body chấp nhận hoặc từ chối lời mời (dùng friendship_id).
type AcceptRejectRequest struct {
	FriendshipID string `json:"friendship_id" binding:"required"`
}
