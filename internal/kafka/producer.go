package kafka

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"github.com/segmentio/kafka-go"
)

// Producer publish MessageEvent lên topic chat.messages.
type Producer struct {
	writer   *kafka.Writer
	produced atomic.Int64
}

// NewProducer tạo producer với Hash balancer — message có cùng Key (conversation_id)
// luôn rơi vào cùng 1 partition, đảm bảo thứ tự tin nhắn trong 1 hội thoại.
func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
		},
	}
}

// PublishMessage gửi event lên Kafka, key = conversation_id.
func (p *Producer) PublishMessage(ctx context.Context, evt MessageEvent) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return p.Publish(ctx, evt.ConversationID, data)
}

// Publish gửi 1 message thô lên topic của Producer này — dùng chung cho mọi loại
// event (chat message, access-gate event...), mỗi loại có Producer/topic riêng.
func (p *Producer) Publish(ctx context.Context, key string, value []byte) error {
	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: value,
	}); err != nil {
		return err
	}

	p.produced.Add(1)
	return nil
}

// ProducedCount trả về tổng số message đã publish thành công — dùng cho trang debug.
func (p *Producer) ProducedCount() int64 { return p.produced.Load() }

func (p *Producer) Close() error { return p.writer.Close() }
