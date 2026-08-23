# internal/kafka

## Đang làm gì?

Học Kafka bằng cách biến luồng "gửi tin nhắn chat" thành 1 pipeline thật đi qua Kafka,
thay vì WebSocket handler tự lưu DB + tự gửi cho người nhận như trước.

```
Browser --WS--> socket.Handler --(producer)--> topic: chat.messages
                                                        │  key = conversation_id
                        ┌───────────────────────────────┴───────────────────────────────┐
                        ▼ consumer group "persist-broadcast"                              ▼ consumer group "notify-unread"
                  lưu Mongo (messageService)                                        chỉ đếm số tin nhắn đã xử lý
                  cập nhật conversation (unread count)                              (đứng vai 1 service khác, ví dụ
                  gọi Hub.DeliverToUsers → phát lại qua WS                           push-notification/analytics sau này)
```

Browser **không** đụng Kafka trực tiếp. Kafka nằm hoàn toàn ở backend, xen giữa
`socket.Handler` (producer) và 2 consumer group độc lập.

Xem trạng thái pipeline lúc đang chạy (đã publish bao nhiêu, mỗi group đọc tới đâu,
lag bao nhiêu) qua `GET /api/v1/kafka/status`, hoặc trang FE `/dev/kafka`.

## Vì sao tách ra như vậy?

Trước đây `socket.Hub.HandleIncoming` làm hết trong 1 goroutine: nhận từ WS → lưu Mongo
→ tự route tới người nhận qua map trong RAM. Chạy tốt với 1 instance, nhưng:

- Không decouple được — muốn thêm 1 tác vụ mới khi có tin nhắn (vd. notification, đếm
  thống kê) là phải sửa thẳng vào Hub.
- Không thấy được các khái niệm cốt lõi của Kafka: producer/consumer, partition, key,
  consumer group, offset, lag, fan-out...

Giờ `socket.Hub` chỉ còn là tầng transport WS thuần (giữ kết nối, đẩy data khi được yêu
cầu) — không còn tự lưu DB. Việc lưu DB + business logic chuyển hết sang consumer.

## File nào làm gì

| File | Vai trò |
|---|---|
| `events.go` | Định nghĩa `MessageEvent` — payload JSON publish lên topic `chat.messages` (conversation_id, sender_id, content). |
| `topic.go` | `EnsureTopic()` — tạo topic nếu chưa có (idempotent), gọi lúc server khởi động qua Admin API (dial tới controller broker rồi `CreateTopics`). Nếu topic đã tồn tại thì bỏ qua lỗi, không crash. |
| `producer.go` | `Producer` bọc `kafka.Writer`. `PublishMessage()` publish 1 `MessageEvent`, dùng **key = conversation_id** với `Hash` balancer → mọi tin nhắn cùng 1 hội thoại luôn rơi vào cùng 1 partition, giữ đúng thứ tự (Kafka chỉ đảm bảo order trong phạm vi 1 partition, không đảm bảo giữa các partition khác nhau). |
| `consumer.go` | `Consumer` bọc `kafka.Reader` theo consumer group — wrapper dùng chung cho cả 2 consumer bên dưới. `Start()` chạy vòng lặp `FetchMessage → handle() → CommitMessages`. Chỉ commit offset **sau khi** xử lý xong → đây là kiểu **at-least-once**: nếu process crash giữa chừng (đã fetch nhưng chưa commit), lần chạy sau sẽ đọc lại message đó (có thể xử lý trùng). |
| `persist_consumer.go` | `PersistBroadcastConsumer` — group `persist-broadcast`, consumer "chính". Nhận `MessageEvent` → build `model.Message`, lưu Mongo (`messageService`) → cập nhật conversation/unread count (`conversationService`) → build lại `WsPayload` (có ID + CreatedAt thật từ DB) → gọi `Hub.DeliverToUsers` để đẩy qua WebSocket tới sender + receiver. Nếu consumer này dừng, tin nhắn vẫn nằm an toàn trong Kafka, không mất — xử lý tiếp khi consumer chạy lại. |
| `notify_consumer.go` | `NotifyConsumer` — group `notify-unread`, **độc lập hoàn toàn** với `persist-broadcast` dù đọc chung 1 topic. Hiện tại chỉ đếm số message đã thấy + log ra, để minh hoạ rõ nhất khái niệm **fan-out**: nhiều consumer group cùng đọc 1 topic, mỗi group nhận đủ toàn bộ message, giữ offset riêng, tắt/bật không ảnh hưởng nhau. Sau này có thể thay bằng 1 service push-notification/analytics thật. |

`socket.Handler.publishIncoming()` (trong `internal/socket/handler.go`) là nơi gọi
`Producer.PublishMessage` — thay cho chỗ trước đây gọi thẳng `messageService.SaveMessageModel`.

## Các khái niệm Kafka đang thấy được qua bài này

- **Topic / Partition / Key** — `chat.messages`, key = `conversation_id`.
- **Producer** — `socket.Handler` publish thay vì gọi DB trực tiếp.
- **Consumer Group** — 2 group (`persist-broadcast`, `notify-unread`) đọc cùng topic,
  độc lập nhau, mỗi group giữ offset riêng.
- **Offset & commit** — commit sau khi xử lý xong (`consumer.go`), tắt/bật consumer sẽ
  đọc tiếp từ offset đã commit lần cuối chứ không đọc lại từ đầu.
- **Lag** — chênh lệch giữa offset mới nhất trên broker và offset consumer đã đọc tới,
  xem qua `reader.Stats().Lag` (expose ở `GET /api/v1/kafka/status`).
- **At-least-once delivery** — commit sau khi handle nên có thể xử lý trùng nếu crash
  giữa chừng; chưa làm dedupe (idempotency) ở bước lưu Mongo vì đây là bài học cơ bản.
- **Admin API** — `EnsureTopic()` tự tạo topic với số partition mong muốn.

## Trạng thái hiện tại

- Đã build + chạy thử thành công (publish → consume) trực tiếp với broker thật ở
  `localhost:9094` — pipeline Kafka hoạt động đúng.
- **Chưa test được full end-to-end qua server thật** vì MongoDB chưa chạy ở máy này
  (`localhost:27017` bị từ chối kết nối) — `go_service` cần Mongo để khởi động
  (xem `cmd/server/main.go` → `config.ConnectMongo`). Cần bật MongoDB (Docker hoặc cài
  local) rồi chạy `go run cmd/server/main.go` để test trọn luồng WS → Kafka → Mongo → WS.
