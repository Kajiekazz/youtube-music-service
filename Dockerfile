FROM python:3.11-alpine

WORKDIR /app

# 安装 yt-dlp 和依赖
RUN apk add --no-cache ffmpeg && \
    pip install --no-cache-dir yt-dlp flask flask-cors requests

# 复制 Python 服务
COPY youtube_service.py .

EXPOSE 8080

CMD ["python", "youtube_service.py"]
