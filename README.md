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
│   └── socket/                 # WebSocket realtime
│       ├── hub.go              # Quản lý clients, broadcast message
│       └── handler.go          # Xử lý kết nối WebSocket từ client
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
- `hub.go` — trung tâm quản lý clients, broadcast.
- `handler.go` — upgrade HTTP → WS, đọc/ghi message.

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
