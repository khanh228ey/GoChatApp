package kafka

import (
	"errors"
	"net"
	"strconv"

	"github.com/segmentio/kafka-go"
)

// EnsureTopic tạo topic nếu chưa tồn tại (idempotent) qua Admin API — dial tới
// controller broker rồi gọi CreateTopics. Nếu topic đã tồn tại thì bỏ qua lỗi.
func EnsureTopic(brokers []string, topic string, partitions, replicationFactor int) error {
	if len(brokers) == 0 {
		return errors.New("kafka: không có broker nào được cấu hình")
	}

	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return err
	}

	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return err
	}
	defer controllerConn.Close()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     partitions,
		ReplicationFactor: replicationFactor,
	})
	if err != nil && !errors.Is(err, kafka.TopicAlreadyExists) {
		return err
	}
	return nil
}
