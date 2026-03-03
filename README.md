# YouTube 音乐服务

YouTube 音频提取服务，支持搜索和播放，部署在 Zeabur 上。

## 功能特性

- ✅ 搜索 YouTube 视频（使用 Invidious API）
- ✅ 获取音频流链接
- ✅ 自动选择最佳音频格式
- ✅ 支持多个 Invidious 实例自动切换

## 文件说明

- `youtube_service.go` - 主程序
- `go.mod` - Go 依赖
- `Dockerfile` - Docker 配置

## 快速部署

### 1. 上传到 GitHub

将此文件夹的所有文件上传到 GitHub 仓库。

### 2. 在 Zeabur 部署

1. 进入 Zeabur 项目（Hong Kong 区域）
2. Add Service → Git
3. 选择此仓库
4. 自动部署

### 3. 生成域名

部署完成后，在 Networking 中生成域名。

## API 使用

### 搜索视频

```
GET /search?q=关键词
```

**返回示例：**
```json
[
  {
    "id": "dQw4w9WgXcQ",
    "title": "视频标题",
    "author": "作者",
    "duration": "3:45",
    "thumbnail": "https://..."
  }
]
```

### 获取音频链接

**通过视频 ID：**
```
GET /audio?id=dQw4w9WgXcQ
```

**通过 URL：**
```
GET /audio?url=https://www.youtube.com/watch?v=dQw4w9WgXcQ
```

**返回示例：**
```json
{
  "url": "https://...",
  "title": "视频标题",
  "author": "作者",
  "duration": "3m45s"
}
```

## 技术栈

- Go 1.21
- github.com/kkdai/youtube/v2 - YouTube 视频提取
- Invidious API - 搜索功能
- Docker Alpine

## 搜索说明

使用 Invidious 公共实例进行搜索：
- 无需 API Key
- 自动切换多个实例
- 返回前 10 条结果

## 注意事项

- 搜索依赖 Invidious 公共实例
- 音频链接有时效性（通常几小时）
- 自动选择最佳音频格式（优先 m4a）

