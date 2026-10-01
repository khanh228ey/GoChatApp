package handler

import (
	"net/http"

	kafkapkg "go_service/internal/kafka"

	"github.com/gin-gonic/gin"
)

// KafkaHandler expose trạng thái pipeline Kafka (producer + các consumer group) —
// dùng cho trang debug FE để "nhìn thấy" Kafka đang hoạt động.
type KafkaHandler struct {
	producer        *kafkapkg.Producer
	persistConsumer *kafkapkg.PersistBroadcastConsumer
	notifyConsumer  *kafkapkg.NotifyConsumer
	accessGate      *kafkapkg.AccessGate
}

func NewKafkaHandler(
	producer *kafkapkg.Producer,
	persistConsumer *kafkapkg.PersistBroadcastConsumer,
	notifyConsumer *kafkapkg.NotifyConsumer,
	accessGate *kafkapkg.AccessGate,
) *KafkaHandler {
	return &KafkaHandler{
		producer:        producer,
		persistConsumer: persistConsumer,
		notifyConsumer:  notifyConsumer,
		accessGate:      accessGate,
	}
}

// GetStatus — GET /api/v1/kafka/status
// Trả snapshot: tổng số message đã publish, và với mỗi consumer group: số message đã
// đọc, lag hiện tại (offset mới nhất trên broker - offset đã đọc), số lần rebalance...
func (h *KafkaHandler) GetStatus(c *gin.Context) {
	persistStats := h.persistConsumer.Stats()
	notifyStats := h.notifyConsumer.Stats()
	accessStats := h.accessGate.Stats()
	activeUsers, waitingUsers := h.accessGate.Snapshot()

	c.JSON(http.StatusOK, gin.H{
		"topic": persistStats.Topic,
		"producer": gin.H{
			"produced_total": h.producer.ProducedCount(),
		},
		"consumer_groups": []gin.H{
			{
				"group_id":       "persist-broadcast",
				"role":           "Lưu MongoDB + phát tin nhắn qua WebSocket",
				"messages_total": persistStats.Messages,
				"lag":            persistStats.Lag,
				"rebalances":     persistStats.Rebalances,
				"errors":         persistStats.Errors,
			},
			{
				"group_id":       "notify-unread",
				"role":           "Đếm tin nhắn độc lập (mô phỏng service notification/analytics)",
				"messages_total": notifyStats.Messages,
				"lag":            notifyStats.Lag,
				"rebalances":     notifyStats.Rebalances,
				"errors":         notifyStats.Errors,
				"handled_total":  h.notifyConsumer.HandledCount(),
			},
		},
		"access_gate": gin.H{
			"topic":          accessStats.Topic,
			"max_slots":      kafkapkg.MaxActiveSlots,
			"active_users":   activeUsers,
			"waiting_users":  waitingUsers,
			"messages_total": accessStats.Messages,
			"lag":            accessStats.Lag,
		},
	})
}
