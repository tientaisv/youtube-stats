# Stage 1: Build Go binary
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build lightweight static Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/server

# Stage 2: Final minimal runtime image
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Copy compiled binary & web frontend assets
COPY --from=builder /app/server /app/server
COPY --from=builder /app/web /app/web

# Expose HTTP port
EXPOSE 9090

# Set environment defaults
ENV PORT=9090
ENV DB_TYPE=sqlite
ENV DB_DSN=/app/data/youtube_stats.db
ENV CRON_SCHEDULE="0 */6 * * *"

# Volume for persistent SQLite DB storage if using SQLite
VOLUME ["/app/data"]

CMD ["/app/server"]
