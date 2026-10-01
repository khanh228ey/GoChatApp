# go_service

Backend Go sử dụng Gin framework, MongoDB và WebSocket.

## Yêu cầu

- Go 1.26+
- MongoDB đang chạy

## Cấu hình

Copy `.env.example` thành `.env` và chỉnh theo môi trường:

```env
PORT=8080
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=go_service_db
JWT_SECRET=your-super-secret-key-change-in-production
JWT_EXPIRE_HOURS=24

KAFKA_BROKERS=localhost:9094
KAFKA_MESSAGES_TOPIC=chat.messages
KAFKA_MESSAGES_PARTITIONS=3
KAFKA_REPLICATION_FACTOR=1
KAFKA_ACCESS_TOPIC=access.events
```

## Chạy server

```bash
go run cmd/server/main.go
```

## Endpoints

| Method | Path                    | Mô tả                          |
|--------|-------------------------|--------------------------------|
| GET    | /ping                   | Health check                   |
| GET    | /ws                     | WebSocket — kết nối realtime   |
| POST   | /api/v1/auth/register   | Đăng ký tài khoản              |
| POST   | /api/v1/auth/login      | Đăng nhập, nhận JWT token      |
| POST   | /api/v1/auth/logout     | Đăng xuất, vô hiệu hóa token   |
| GET    | /api/v1/kafka/status    | Trạng thái pipeline Kafka (producer + consumer group) |

### Auth API

**Đăng ký** — `POST /api/v1/auth/register`

```json
{
  "email": "user@example.com",
  "phone": "0901234567",
  "password": "123456"
}
```

Cần ít nhất `email` hoặc `phone`. Password tối thiểu 6 ký tự.

**Đăng nhập** — `POST /api/v1/auth/login`

```json
{
  "identifier": "user@example.com",
  "password": "123456"
}
```

`identifier` có thể là email hoặc số điện thoại.

**Đăng xuất** — `POST /api/v1/auth/logout`

```
Authorization: Bearer <token>
```

---

## Cấu trúc thư mục

```
go_service/
├── cmd/                        # Entry point — chỉ chứa hàm main()
│   └── server/
│       └── main.go             # Khởi động server: config → DB → socket → HTTP
│
├── internal/                   # Code nội bộ, không export ra ngoài module
│   ├── config/                 # Cấu hình ứng dụng
│   │   ├── config.go           # Đọc biến môi trường (.env)
│   │   └── database.go         # Kết nối / ngắt kết nối MongoDB
│   │
│   ├── middleware/             # HTTP middleware dùng chung
│   │   └── cors.go             # CORS — cho phép frontend gọi API
│   │
│   ├── routes/                 # Đăng ký tất cả HTTP routes
│   │   └── routes.go           # Gom endpoint vào 1 chỗ, gọi từ main.go
│   │
│   ├── socket/                 # WebSocket realtime
│   │   ├── hub.go              # Quản lý clients, phát message tới đúng user
│   │   └── handler.go          # Xử lý kết nối WS, publish tin nhắn lên Kafka
│   │
│   └── kafka/                  # Kafka producer/consumer — xem internal/kafka/README.md
│       ├── producer.go         # Publish MessageEvent lên topic chat.messages
│       ├── consumer.go         # Wrapper chung cho kafka.Reader theo consumer group
│       ├── persist_consumer.go # Group "persist-broadcast": lưu Mongo + phát WS
│       ├── notify_consumer.go  # Group "notify-unread": đếm tin nhắn (demo fan-out)
│       ├── access_gate.go      # Giới hạn tối đa 2 user online (1 partition, 1 consumer)
│       └── topic.go            # Tạo topic idempotent qua Admin API
│
├── .env                        # Biến môi trường (không commit)
├── .env.example                # Mẫu biến môi trường
├── go.mod                      # Module và dependencies
└── go.sum                      # Checksum dependencies
```

---

## Tác dụng từng folder

### `cmd/`
Chứa **entry point** của ứng dụng. Mỗi binary (server, worker, CLI...) có 1 subfolder riêng.
- `cmd/server/main.go` — chỉ làm wiring: load config, connect DB, khởi tạo service, chạy server.
- **Không** viết business logic ở đây.

### `internal/config/`
Quản lý **cấu hình** và **kết nối database**.
- `config.go` — đọc `.env`, trả về struct `Config`.
- `database.go` — connect/disconnect MongoDB.

### `internal/middleware/`
Các **middleware HTTP** áp dụng cho mọi request (CORS, auth, logging...).
- Thêm middleware mới vào đây, đăng ký trong `routes.Setup()`.

### `internal/routes/`
**Đăng ký routes** — gom tất cả endpoint vào 1 file.
- API REST, WebSocket, health check đều khai báo ở đây.
- `main.go` chỉ gọi `routes.Setup()`.

### `internal/socket/`
Xử lý **WebSocket realtime**.
- `hub.go` — trung tâm quản lý clients, phát data tới user (không tự lưu DB nữa).
- `handler.go` — upgrade HTTP → WS, publish tin nhắn nhận được lên Kafka.

### `internal/kafka/`
Pipeline tin nhắn qua **Kafka** — tách việc "nhận tin nhắn từ WS" ra khỏi việc "lưu DB
+ phát tới người nhận", để hiểu producer/consumer/consumer group hoạt động thế nào.

```
Browser --WS--> socket.Handler --producer--> topic: chat.messages (key = conversation_id)
                                                        │
                        ┌───────────────────────────────┴───────────────────────────────┐
                        ▼ group "persist-broadcast"                                       ▼ group "notify-unread"
                  lưu Mongo + gọi Hub.DeliverToUsers                                đếm tin nhắn (demo fan-out,
                  → phát lại qua WebSocket tới sender/receiver                       mô phỏng service notification)
```

- Key Kafka message = `conversation_id` → mọi tin nhắn cùng 1 hội thoại luôn vào cùng
  partition, giữ đúng thứ tự (Kafka chỉ đảm bảo order trong phạm vi 1 partition).
- 2 consumer group đọc **độc lập** cùng 1 topic — minh họa fan-out: mỗi group nhận đủ
  toàn bộ message, giữ offset riêng, dừng/khởi động lại không ảnh hưởng group kia.
- Xem trạng thái pipeline (số message đã publish/consume, lag từng group) qua
  `GET /api/v1/kafka/status` hoặc trang debug FE `/dev/kafka`.

---

## Folder sẽ thêm sau (khi làm feature)

| Folder                  | Tác dụng                                      |
|-------------------------|-----------------------------------------------|
| `internal/handler/`     | HTTP handlers — nhận request, trả response    |
| `internal/service/`     | Business logic — xử lý nghiệp vụ              |
| `internal/repository/`  | Truy vấn database (MongoDB collections)       |
| `internal/model/`       | Struct đại diện document trong DB             |
| `internal/dto/`         | Struct request/response cho API               |

### Luồng xử lý khi thêm feature mới

```
Request → routes → handler → service → repository → MongoDB
                              ↓
                         socket/hub (nếu cần realtime)
```
