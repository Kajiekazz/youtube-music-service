package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kkdai/youtube/v2"
)

type SearchResult struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Duration string `json:"duration"`
	Thumbnail string `json:"thumbnail"`
}

type AudioInfo struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Duration string `json:"duration"`
}

func main() {
	http.HandleFunc("/search", handleSearch)
	http.HandleFunc("/audio", handleAudio)
	http.HandleFunc("/", handleRoot)

	port := "8080"
	log.Printf("YouTube 音乐服务启动在端口 %s", port)
	log.Printf("搜索: /search?q=关键词")
	log.Printf("获取音频: /audio?id=视频ID")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "YouTube 音乐提取服务\n\n")
	fmt.Fprintf(w, "使用方法:\n")
	fmt.Fprintf(w, "1. 搜索: /search?q=关键词\n")
	fmt.Fprintf(w, "2. 获取音频: /audio?id=视频ID\n")
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error":"缺少搜索关键词"}`, http.StatusBadRequest)
		return
	}

	log.Printf("搜索请求: %s", query)

	// 使用 Invidious API 搜索
	results, err := searchViaInvidious(query)
	if err != nil {
		log.Printf("搜索失败: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"搜索失败: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	log.Printf("搜索成功，返回 %d 条结果", len(results))
	json.NewEncoder(w).Encode(results)
}

// Invidious 搜索响应结构
type invidiousSearchResult struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	VideoID       string `json:"videoId"`
	Author        string `json:"author"`
	LengthSeconds int    `json:"lengthSeconds"`
	VideoThumbnails []struct {
		URL string `json:"url"`
	} `json:"videoThumbnails"`
}

func searchViaInvidious(query string) ([]SearchResult, error) {
	// Invidious 公共实例列表（按优先级）
	instances := []string{
		"https://inv.nadeko.net",
		"https://invidious.jing.rocks",
		"https://invidious.privacyredirect.com",
		"https://y.com.sb",
	}

	var lastErr error
	for _, instance := range instances {
		results, err := tryInvidiousInstance(instance, query)
		if err == nil {
			return results, nil
		}
		lastErr = err
		log.Printf("实例 %s 失败: %v，尝试下一个...", instance, err)
	}

	return nil, fmt.Errorf("所有 Invidious 实例都失败了: %v", lastErr)
}

func tryInvidiousInstance(instance, query string) ([]SearchResult, error) {
	// 构造搜索 URL
	searchURL := fmt.Sprintf("%s/api/v1/search?q=%s&type=video", instance, url.QueryEscape(query))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(searchURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var invResults []invidiousSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&invResults); err != nil {
		return nil, err
	}

	// 转换为我们的格式
	results := make([]SearchResult, 0, len(invResults))
	for _, item := range invResults {
		if item.Type != "video" {
			continue
		}

		// 格式化时长
		duration := formatDuration(item.LengthSeconds)

		// 获取缩略图
		thumbnail := ""
		if len(item.VideoThumbnails) > 0 {
			thumbnail = item.VideoThumbnails[0].URL
		}

		results = append(results, SearchResult{
			ID:        item.VideoID,
			Title:     item.Title,
			Author:    item.Author,
			Duration:  duration,
			Thumbnail: thumbnail,
		})

		// 限制返回 10 条结果
		if len(results) >= 10 {
			break
		}
	}

	return results, nil
}

func formatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

func handleAudio(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	videoID := r.URL.Query().Get("id")
	videoURL := r.URL.Query().Get("url")

	if videoID == "" && videoURL == "" {
		http.Error(w, `{"error":"缺少视频ID或URL"}`, http.StatusBadRequest)
		return
	}

	// 如果提供的是 URL，提取 ID
	if videoURL != "" {
		id, err := extractVideoID(videoURL)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"无效的YouTube URL: %s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		videoID = id
	}

	log.Printf("获取音频: %s", videoID)

	// 创建 YouTube 客户端
	client := youtube.Client{}

	// 获取视频信息
	video, err := client.GetVideo(videoID)
	if err != nil {
		log.Printf("获取视频失败: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"获取视频失败: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// 获取音频格式
	formats := video.Formats.WithAudioChannels()
	if len(formats) == 0 {
		http.Error(w, `{"error":"没有找到音频流"}`, http.StatusNotFound)
		return
	}

	// 选择最佳音频格式（优先选择 m4a 或 webm）
	var bestFormat *youtube.Format
	for i := range formats {
		format := &formats[i]
		if format.MimeType != "" {
			// 优先选择纯音频格式
			if strings.Contains(format.MimeType, "audio/mp4") ||
			   strings.Contains(format.MimeType, "audio/webm") {
				if bestFormat == nil || format.Bitrate > bestFormat.Bitrate {
					bestFormat = format
				}
			}
		}
	}

	// 如果没有纯音频，选择任意有音频的格式
	if bestFormat == nil {
		bestFormat = &formats[0]
	}

	// 获取下载 URL
	downloadURL, err := client.GetStreamURL(video, bestFormat)
	if err != nil {
		log.Printf("获取下载链接失败: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"获取下载链接失败: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// 返回音频信息
	info := AudioInfo{
		URL:      downloadURL,
		Title:    video.Title,
		Author:   video.Author,
		Duration: video.Duration.String(),
	}

	log.Printf("成功获取音频: %s - %s", video.Title, video.Author)
	json.NewEncoder(w).Encode(info)
}

func extractVideoID(videoURL string) (string, error) {
	u, err := url.Parse(videoURL)
	if err != nil {
		return "", err
	}

	// 处理不同的 YouTube URL 格式
	// https://www.youtube.com/watch?v=VIDEO_ID
	// https://youtu.be/VIDEO_ID
	// https://m.youtube.com/watch?v=VIDEO_ID

	if u.Host == "youtu.be" {
		// 短链接格式
		return strings.TrimPrefix(u.Path, "/"), nil
	}

	// 标准格式
	query := u.Query()
	videoID := query.Get("v")
	if videoID == "" {
		return "", fmt.Errorf("无法从URL中提取视频ID")
	}

	return videoID, nil
}
