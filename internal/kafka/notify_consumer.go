package kafka

import (
	"context"
	"encoding/json"
	"log"
	"sync/atomic"

	"github.com/segmentio/kafka-go"
)

// NotifyConsumer là 1 consumer group ĐỘC LẬP đọc cùng topic chat.messages với
// PersistBroadcastConsumer — minh hoạ fan-out: nhiều consumer group cùng đọc 1 topic,
// mỗi group nhận đủ toàn bộ message và giữ offset riêng, không ảnh hưởng lẫn nhau.
// Ở đây chỉ đếm số tin nhắn (đứng vai một service push-notification/analytics sau này).
type NotifyConsumer struct {
	consumer *Consumer
	handled  atomic.Int64
}

func NewNotifyConsumer(brokers []string, topic string) *NotifyConsumer {
	return &NotifyConsumer{
		consumer: NewConsumer(brokers, topic, "notify-unread", "notify-unread"),
	}
}

func (n *NotifyConsumer) Start(ctx context.Context) {
	n.consumer.Start(ctx, n.handle)
}

func (n *NotifyConsumer) Stats() kafka.ReaderStats { return n.consumer.Stats() }

// Close đóng reader — gửi LeaveGroup cho broker để group được rebalance ngay.
func (n *NotifyConsumer) Close() error { return n.consumer.Close() }

// HandledCount trả về tổng số message consumer này đã xử lý — dùng cho trang debug.
func (n *NotifyConsumer) HandledCount() int64 { return n.handled.Load() }

func (n *NotifyConsumer) handle(_ context.Context, m kafka.Message) error {
	var evt MessageEvent
	if err := json.Unmarshal(m.Value, &evt); err != nil {
		return err
	}
	n.handled.Add(1)
	log.Printf("[kafka:notify-unread] tin nhắn mới conv=%s sender=%s offset=%d", evt.ConversationID, evt.SenderID, m.Offset)
	return nil
}
