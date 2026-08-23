// Package kafka chứa producer/consumer cho pipeline tin nhắn qua Kafka:
// WS handler publish → topic chat.messages → 2 consumer group đọc độc lập
// (persist-broadcast: lưu Mongo + phát WS; notify-unread: đếm tin nhắn, mô phỏng service notification).
package kafka

// MessageEvent là payload publish lên topic chat.messages khi có tin nhắn mới.
// Key của Kafka message = ConversationID để mọi tin nhắn cùng hội thoại vào cùng 1 partition,
// giữ đúng thứ tự (Kafka chỉ đảm bảo order trong phạm vi 1 partition).
type MessageEvent struct {
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	Content        string `json:"content"`
}
