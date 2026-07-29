# 🚀 YouTube Video Statistics & Analytics Manager

Hệ thống theo dõi, quản lý và phân tích số liệu kênh & video YouTube tự động viết bằng **Go (Golang)**, tích hợp giao diện **Web Dashboard** hiện đại và cơ sở dữ liệu **PostgreSQL**.

---

## 🌟 Tính năng chính

- 📊 **Dashboard Phân Tích**: Theo dõi tổng quan lượt xem, lượt thích, bình luận, và đăng ký theo thời gian thực.
- 📹 **Quản lý Video & Kênh**: Theo dõi lịch sử phát triển của từng video và kênh YouTube (bao gồm cả kênh của bạn và đối thủ).
- 🕒 **Hourly Peak Stats**: Phân tích khung giờ vàng có lượng view tăng trưởng cao nhất trong ngày.
- 🥊 **Competitor Comparison**: So sánh chỉ số tương tác (Engagement Rate) và tần suất đăng bài (Posting Frequency) với các kênh đối thủ.
- 🔑 **API Key Rotation Pool**: Tự động xoay vòng nhiều YouTube API Key để tránh chạm giới hạn Quota hàng ngày.
- ⏱️ **Cron Scheduler**: Tự động thu thập dữ liệu định kỳ (mặc định 6 tiếng/lần).
- 🔐 **Phân quyền người dùng**: Hệ thống tài khoản phân quyền Admin & SuperAdmin.

---

## 🛠️ Công nghệ sử dụng

- **Backend**: Go (Golang 1.22+)
- **Database**: PostgreSQL 15 / SQLite
- **Frontend**: HTML5, CSS3 (Vanilla Dark Mode UI), JavaScript (ES6+), Chart.js
- **Container**: Docker & Docker Compose

---

## 🚀 Hướng dẫn cài đặt & Chạy ứng dụng

### 1. Yêu cầu hệ thống
- Docker & Docker Compose
- Hoặc Go >= 1.22 (nếu muốn chạy trực tiếp không qua Docker)

### 2. Khởi chạy nhanh với Docker Compose (Khuyên dùng)

1. **Clone Repository:**
   ```bash
   git clone https://github.com/tientaisv/youtube-stats.git
   cd youtube-stats
   ```

2. **Cấu hình môi trường (`.env`):**
   Tạo file `.env` từ `.env.example` hoặc chỉnh sửa các tham số mặc định:
   ```env
   PORT=9090
   DB_TYPE=postgres
   DB_DSN=postgres://postgres:postgrespassword@postgres:5432/youtube_stats?sslmode=disable
   YOUTUBE_API_KEYS=YOUR_YOUTUBE_API_KEY_1,YOUR_YOUTUBE_API_KEY_2
   CRON_SCHEDULE="0 */6 * * *"
   ```

3. **Chạy ứng dụng với Docker Compose:**
   ```bash
   docker compose up -d --build
   ```

4. **Truy cập ứng dụng:**
   Mở trình duyệt và truy cập: **`http://localhost:9090`**

---

## 🔐 Tài khoản mặc định

Sau khi khởi tạo cơ sở dữ liệu lần đầu, hệ thống sẽ tự động tạo tài khoản mặc định:

- **Admin Account**:
  - Username: `admin`
  - Password: `admin123`

---

## 📁 Cấu trúc thư mục dự án

```text
.
├── cmd/
│   └── server/          # Main entrypoint cho Go application
├── internal/
│   ├── api/             # HTTP Handlers & Routers
│   ├── collector/       # Logic thu thập số liệu YouTube
│   ├── config/          # Cấu hình ứng dụng
│   ├── database/        # Kết nối & Migration DB (Postgres/SQLite)
│   ├── models/          # Data structs & Models
│   ├── scheduler/       # Cron job quản lý lịch thu thập
│   └── youtube/         # Client tương tác với YouTube Data API v3
├── web/                 # Web Frontend Static (HTML, CSS, JS)
├── Dockerfile           # Docker build configuration
├── docker-compose.yml   # Docker Compose services definition
├── go.mod               # Go module dependencies
└── README.md
```

---

## 📄 Trạng thái Port mặc định

- **Web App**: `9090`
- **PostgreSQL Database**: Expose port `5435` trên host (đã map vào port `5432` trong container để tránh xung đột).

---

## 📝 License

Dự án này phục vụ mục đích cá nhân và quản lý dữ liệu YouTube.
