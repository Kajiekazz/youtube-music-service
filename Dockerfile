FROM golang:1.21-alpine AS builder

WORKDIR /app

# 复制 go.mod 和代码
COPY go.mod .
COPY go.sum .
COPY youtube_service.go .

# 下载依赖
RUN go mod download

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -o youtube-service youtube_service.go

# 最终镜像
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/youtube-service .

EXPOSE 8080

CMD ["./youtube-service"]
