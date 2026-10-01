# internal/kafka

## Đang làm gì?

Học Kafka bằng cách xây 2 tính năng thật đi qua Kafka thay vì xử lý trực tiếp trong
WebSocket handler như trước:

1. **Pipeline tin nhắn** (`events.go`, `producer.go`, `consumer.go`, `persist_consumer.go`,
   `notify_consumer.go`) — minh hoạ **fan-out**: nhiều partition, nhiều consumer group
   đọc song song cùng 1 topic.
2. **AccessGate — giới hạn 2 user online** (`access_gate.go`) — minh hoạ pattern ngược
   lại: **1 partition, đúng 1 consumer**, dùng Kafka làm "single-writer coordinator" để
   tránh race condition khi nhiều connection cùng connect/disconnect một lúc. Xem mục
   riêng bên dưới.

## 1. Pipeline tin nhắn chat

Biến luồng "gửi tin nhắn chat" thành 1 pipeline thật đi qua Kafka, thay vì WebSocket
handler tự lưu DB + tự gửi cho người nhận như trước.

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

## Các khái niệm Kafka đang thấy được qua phần 1 (pipeline tin nhắn)

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

## 2. AccessGate — giới hạn 2 user online cùng lúc

Yêu cầu: cả app chỉ cho tối đa **2 user online cùng lúc**. Người thứ 3 kết nối vào sẽ
thấy màn hình "đang chờ" (FE), và tự động được vào ngay khi 1 trong 2 người kia rời đi
— không cần tải lại trang.

```
Browser --WS--> socket.Handler
                    │  publish AccessEvent{connect, user_id}
                    ▼
            topic: access.events   ← CHỈ 1 PARTITION
                    │
                    ▼
       consumer group "access-gate"  ← CHỈ 1 CONSUMER
       (giữ active[] tối đa 2 user + queue[] chờ, trong RAM)
                    │
        còn chỗ? → đóng channel chờ của user đó → WS handler
        cho user vào Hub (online thật) + gửi "access_granted"
        hết chỗ? → thêm vào queue, WS handler vẫn giữ kết nối
        mở nhưng KHÔNG cho vào Hub, gửi "waiting"
```

Khác hẳn `chat.messages` (nhiều partition, nhiều consumer group đọc song song), topic
`access.events` **luôn chỉ có 1 partition** và **chỉ 1 consumer group** được phép chạy.
Nhờ đó mọi sự kiện connect/disconnect (từ bất kỳ WS connection nào) được xử lý theo
đúng thứ tự thời gian thực xảy ra — không có 2 goroutine nào cùng lúc sửa `active`/`queue`,
nên không cần lock phức tạp hay lo race condition dù có bao nhiêu người connect cùng lúc.
Đây là pattern **Kafka làm single-writer coordinator**: dùng chính log có thứ tự của
Kafka làm nguồn sự thật duy nhất, thay vì để nhiều goroutine tự tranh chấp 1 biến chung.

**Cơ chế "chờ" hoạt động thế nào (`internal/socket/handler.go`):**

- Khi WS connect: server luôn nâng cấp kết nối HTTP→WS bình thường (để có thể gửi status
  ngay), rồi gọi `AccessGate.RequestAccess()` — hàm này đăng ký 1 channel chờ **rồi mới**
  publish sự kiện `connect` (đăng ký trước để không bỏ lỡ tín hiệu cấp slot do timing).
- Client được gửi `{"type":"waiting"}` ngay lập tức. Nếu còn chỗ, consumer xử lý gần như
  tức thì và đóng channel chờ → WS handler gọi `Hub.RegisterClient` (giờ mới thực sự
  "online", nhận/gửi được chat) và gửi `{"type":"access_granted"}`.
- Trong lúc chờ, vòng lặp đọc WS vẫn chạy (để phát hiện disconnect ngay), chỉ là bỏ qua
  nội dung tin nhắn gửi lên trước khi được cấp slot.
- Khi WS đóng (dù đang online hay đang chờ): publish sự kiện `disconnect`. Consumer xoá
  user khỏi `active`/`queue`; nếu vừa giải phóng 1 chỗ và còn người trong `queue`, cấp
  slot ngay cho người đầu hàng.

**File:**

| File | Vai trò |
|---|---|
| `access_gate.go` | `AccessGate` — toàn bộ logic trên: `RequestAccess()`, `Release()`, `handle()` (nơi DUY NHẤT được sửa `active`/`queue`), `Snapshot()` (cho trang debug). |

**Lưu ý vận hành:** vì `access-gate` chỉ chạy 1 consumer, nếu server bị kill đột ngột
(không qua `Shutdown()`) thì lần khởi động sau phải chờ hết `SessionTimeout` (mặc định
30s của kafka-go) trước khi consumer mới join lại được group — trong lúc đó mọi user sẽ
bị kẹt ở màn hình chờ. `cmd/server/main.go` đã bắt `SIGINT`/`SIGTERM` để gọi
`App.Shutdown()` (đóng consumer đúng cách, gửi `LeaveGroup`) trước khi thoát — nên dừng
server bằng Ctrl+C (không phải `kill -9`) để tránh việc này khi dev.

**Giới hạn hiện tại (chấp nhận được cho bài học, không phải production-ready):**
`active`/`queue` chỉ nằm trong RAM của `AccessGate` — nếu restart server, trạng thái
này mất trắng dù Kafka vẫn còn nguyên log sự kiện (khác với `persist-broadcast`, nơi
Mongo mới là nguồn lưu trữ lâu dài, Kafka chỉ là transport tạm). Muốn đúng nghĩa "event
sourcing" hơn thì lúc khởi động, consumer phải đọc lại từ đầu topic (`StartOffset:
FirstOffset`) để dựng lại `active`/`queue` trước khi xử lý message mới — chưa làm bước
này.

## Các khái niệm Kafka đang thấy được qua phần 2 (AccessGate)

- **Partition = đơn vị thứ tự** — Kafka chỉ đảm bảo order trong 1 partition; ép topic
  chỉ 1 partition là cách "mua" total ordering cho toàn bộ sự kiện, đánh đổi bằng việc
  không scale được (mãi mãi chỉ 1 consumer xử lý được, không thể chia tải).
- **Consumer Group làm single-writer** — thay vì nhiều goroutine tranh nhau sửa 1 biến
  dùng chung (cần mutex phức tạp, dễ sai), để đúng 1 consumer duy nhất làm "người viết
  duy nhất", các nơi khác chỉ publish sự kiện rồi chờ kết quả.
- **Kafka log làm nguồn sự thật (source of truth)** — quyết định "ai được vào" không do
  goroutine nào tự tính toán, mà do thứ tự sự kiện đã ghi trên log quyết định.
- **Trade-off**: đổi lấy sự an toàn/đúng đắn, bài học này chấp nhận thêm 1 vòng round-trip
  qua Kafka cho mỗi lần connect/disconnect (thường vài chục ms) — với 1 tính năng chỉ cần
  đúng cho 1 instance, `sync.Mutex` thuần Go sẽ đơn giản và nhanh hơn nhiều (xem lại lựa
  chọn ban đầu ở phần mô tả yêu cầu).

## Trạng thái hiện tại

- **Pipeline tin nhắn**: đã build + chạy thử thành công (publish → consume) trực tiếp
  với broker thật ở `localhost:9094`.
- **AccessGate**: đã viết 1 chương trình smoke test riêng (không commit vào repo) mô
  phỏng 3 user connect liên tiếp — xác nhận đúng: user 1 và 2 được cấp slot ngay, user 3
  phải chờ (`queue=1`), sau khi user 1 rời đi thì user 3 được cấp slot ngay lập tức,
  `Snapshot()` phản ánh đúng trạng thái ở mọi bước.
- **Chưa test được full end-to-end qua server thật** (WS thật từ browser → AccessGate →
  Hub → chat) vì MongoDB chưa chạy ở máy này (`localhost:27017` bị từ chối kết nối) —
  `go_service` cần Mongo để khởi động (xem `cmd/server/main.go` → `config.ConnectMongo`).
  Cần bật MongoDB (Docker hoặc cài local) rồi chạy `go run cmd/server/main.go` để test
  trọn luồng, kể cả mở 3 tab trình duyệt để thấy tab thứ 3 vào màn hình chờ.
