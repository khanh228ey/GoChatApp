package kafka

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/segmentio/kafka-go"
)

// MaxActiveSlots là số user tối đa được "vào" cùng lúc — người vượt quá phải xếp hàng chờ.
const MaxActiveSlots = 2

// AccessEventType là loại sự kiện trên topic access.events.
type AccessEventType string

const (
	AccessConnect    AccessEventType = "connect"
	AccessDisconnect AccessEventType = "disconnect"
)

// AccessEvent là payload publish lên topic access.events khi 1 WS connection mở/đóng.
type AccessEvent struct {
	Type   AccessEventType `json:"type"`
	UserID string          `json:"user_id"`
}

// AccessGate giới hạn toàn app chỉ MaxActiveSlots user online cùng lúc, còn lại xếp
// hàng chờ. Khác với chat.messages (nhiều partition, nhiều consumer group đọc song
// song), topic access.events CHỈ CÓ 1 PARTITION và chỉ được xử lý bởi ĐÚNG 1 consumer
// (group "access-gate") — nhờ vậy mọi connect/disconnect được xử lý TUẦN TỰ, theo đúng
// thứ tự thời gian thực xảy ra, nên không có race condition dù nhiều WS connection
// cùng connect/disconnect một lúc. Đây là pattern "Kafka làm single-writer coordinator" —
// dùng chính log có thứ tự của Kafka làm nguồn sự thật duy nhất cho trạng thái active/queue,
// thay vì để nhiều goroutine tự tranh chấp 1 biến dùng chung.
type AccessGate struct {
	producer *Producer
	consumer *Consumer

	mu      sync.Mutex
	active  map[string]struct{}      // userID đang chiếm 1 trong MaxActiveSlots chỗ
	queue   []string                 // userID đang xếp hàng chờ, theo thứ tự FIFO
	waiters map[string]chan struct{} // userID -> channel, được đóng lại khi user đó được cấp slot
}

func NewAccessGate(brokers []string, topic string) *AccessGate {
	return &AccessGate{
		producer: NewProducer(brokers, topic),
		consumer: NewConsumer(brokers, topic, "access-gate", "access-gate"),
		active:   make(map[string]struct{}, MaxActiveSlots),
		waiters:  make(map[string]chan struct{}),
	}
}

// Start chạy consumer loop, block tới khi ctx bị huỷ.
func (g *AccessGate) Start(ctx context.Context) {
	g.consumer.Start(ctx, g.handle)
}

func (g *AccessGate) Stats() kafka.ReaderStats { return g.consumer.Stats() }

// Close đóng reader — gửi LeaveGroup cho broker để group "access-gate" được rebalance
// ngay khi server restart, tránh phải chờ hết SessionTimeout (mặc định 30s) mới join lại được.
func (g *AccessGate) Close() error { return g.consumer.Close() }

// Snapshot trả về danh sách userID đang active / đang chờ — dùng cho trang debug.
func (g *AccessGate) Snapshot() (active []string, queue []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	active = make([]string, 0, len(g.active))
	for id := range g.active {
		active = append(active, id)
	}
	queue = append(queue, g.queue...)
	return active, queue
}

// RequestAccess đăng ký 1 "waiter" cho userID rồi publish sự kiện connect. Channel trả
// về sẽ được consumer đóng lại (từ handle()) khi user này được cấp slot — có thể đóng
// gần như ngay lập tức nếu còn chỗ trống, hoặc sau khi có người khác rời đi.
func (g *AccessGate) RequestAccess(ctx context.Context, userID string) <-chan struct{} {
	ch := make(chan struct{})

	g.mu.Lock()
	g.waiters[userID] = ch
	g.mu.Unlock()

	if err := g.publish(ctx, AccessEvent{Type: AccessConnect, UserID: userID}); err != nil {
		log.Printf("[kafka:access-gate] publish connect failed, cấp quyền tạm để không kẹt user: %v", err)
		g.mu.Lock()
		delete(g.waiters, userID)
		g.mu.Unlock()
		close(ch)
	}

	return ch
}

// Release publish sự kiện disconnect cho userID — gọi khi WS connection đóng, dù
// user đó đang active hay đang trong hàng chờ.
func (g *AccessGate) Release(ctx context.Context, userID string) {
	if err := g.publish(ctx, AccessEvent{Type: AccessDisconnect, UserID: userID}); err != nil {
		log.Printf("[kafka:access-gate] publish disconnect failed: %v", err)
	}
}

func (g *AccessGate) publish(ctx context.Context, evt AccessEvent) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	// Key = userID: topic chỉ 1 partition nên key nào cũng vào partition 0, nhưng vẫn
	// đặt key rõ nghĩa để nếu sau này lỡ tăng partition thì vẫn còn ý nghĩa định tuyến.
	return g.producer.Publish(ctx, evt.UserID, data)
}

// handle xử lý TUẦN TỰ từng AccessEvent — đây là nơi DUY NHẤT được phép sửa active/queue.
func (g *AccessGate) handle(_ context.Context, m kafka.Message) error {
	var evt AccessEvent
	if err := json.Unmarshal(m.Value, &evt); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	switch evt.Type {
	case AccessConnect:
		if _, ok := g.active[evt.UserID]; ok {
			// Đã active sẵn (vd. reconnect) → cấp lại ngay cho waiter mới đăng ký, idempotent.
			g.grantLocked(evt.UserID)
			return nil
		}
		if len(g.active) < MaxActiveSlots {
			g.active[evt.UserID] = struct{}{}
			g.grantLocked(evt.UserID)
			log.Printf("[kafka:access-gate] %s vào — active=%d/%d", evt.UserID, len(g.active), MaxActiveSlots)
		} else if !containsString(g.queue, evt.UserID) {
			g.queue = append(g.queue, evt.UserID)
			log.Printf("[kafka:access-gate] %s phải chờ — queue=%d", evt.UserID, len(g.queue))
		}

	case AccessDisconnect:
		wasActive := false
		if _, ok := g.active[evt.UserID]; ok {
			delete(g.active, evt.UserID)
			wasActive = true
		}
		g.queue = removeString(g.queue, evt.UserID)
		delete(g.waiters, evt.UserID) // nếu đang chờ mà rời đi → dọn waiter, không cấp slot nữa

		if wasActive {
			log.Printf("[kafka:access-gate] %s rời đi — active=%d/%d", evt.UserID, len(g.active), MaxActiveSlots)
		}

		if len(g.active) < MaxActiveSlots && len(g.queue) > 0 {
			next := g.queue[0]
			g.queue = g.queue[1:]
			g.active[next] = struct{}{}
			g.grantLocked(next)
			log.Printf("[kafka:access-gate] %s được cấp slot từ hàng chờ — active=%d/%d", next, len(g.active), MaxActiveSlots)
		}
	}
	return nil
}

// grantLocked đóng channel chờ của userID (nếu có) để đánh thức goroutine WS đang
// block chờ. Phải gọi khi đang giữ g.mu.
func (g *AccessGate) grantLocked(userID string) {
	if ch, ok := g.waiters[userID]; ok {
		close(ch)
		delete(g.waiters, userID)
	}
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func removeString(s []string, v string) []string {
	for i, x := range s {
		if x == v {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
