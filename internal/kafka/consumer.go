package kafka

import (
	"context"
	"log"

	"github.com/segmentio/kafka-go"
)

// Consumer bọc kafka.Reader theo consumer group. Nhiều Consumer trỏ cùng Topic
// nhưng khác GroupID sẽ cùng nhận được mọi message (fan-out) — mỗi group giữ offset riêng.
type Consumer struct {
	Name    string
	GroupID string
	reader  *kafka.Reader
}

func NewConsumer(brokers []string, topic, groupID, name string) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	return &Consumer{Name: name, GroupID: groupID, reader: reader}
}

// Start chạy vòng lặp fetch → handle → commit offset, block tới khi ctx bị huỷ.
// Offset chỉ được commit SAU khi handle xong — nếu process crash giữa chừng,
// message sẽ được đọc lại ở lần chạy sau (at-least-once, không phải exactly-once).
func (c *Consumer) Start(ctx context.Context, handle func(ctx context.Context, m kafka.Message) error) {
	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[kafka:%s] fetch error: %v", c.Name, err)
			continue
		}

		if err := handle(ctx, m); err != nil {
			log.Printf("[kafka:%s] handle error (bỏ qua, vẫn commit offset): %v", c.Name, err)
		}

		if err := c.reader.CommitMessages(ctx, m); err != nil {
			log.Printf("[kafka:%s] commit error: %v", c.Name, err)
		}
	}
}

// Stats trả snapshot thống kê reader (offset, lag, số message đã đọc...) — dùng cho trang debug.
func (c *Consumer) Stats() kafka.ReaderStats { return c.reader.Stats() }

func (c *Consumer) Close() error { return c.reader.Close() }
