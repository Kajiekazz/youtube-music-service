package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

type SearchResult struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Duration  string `json:"duration"`
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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error":"缺少搜索关键词"}`, http.StatusBadRequest)
		return
	}

	log.Printf("搜索请求: %s", query)

	results, err := searchViaInvidious(query)
	if err != nil {
		log.Printf("搜索失败: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"搜索失败: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	log.Printf("搜索成功，返回 %d 条结果", len(results))
	json.NewEncoder(w).Encode(results)
}

func handleAudio(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	videoID := r.URL.Query().Get("id")
	videoURL := r.URL.Query().Get("url")

	if videoID == "" && videoURL == "" {
		http.Error(w, `{"error":"缺少视频ID或URL"}`, http.StatusBadRequest)
		return
	}

	if videoURL != "" {
		id, err := extractVideoID(videoURL)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"无效的YouTube URL: %s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		videoID = id
	}

	log.Printf("获取音频: %s", videoID)

	// 使用 Invidious API 获取音频
	audioInfo, err := getAudioViaInvidious(videoID)
	if err != nil {
		log.Printf("获取音频失败: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"获取音频失败: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	log.Printf("成功获取音频: %s - %s", audioInfo.Title, audioInfo.Author)
	json.NewEncoder(w).Encode(audioInfo)
}

// Invidious 搜索响应结构
type invidiousSearchResult struct {
	Type            string `json:"type"`
	Title           string `json:"title"`
	VideoID         string `json:"videoId"`
	Author          string `json:"author"`
	LengthSeconds   int    `json:"lengthSeconds"`
	VideoThumbnails []struct {
		URL string `json:"url"`
	} `json:"videoThumbnails"`
}

// Invidious 视频信息响应
type invidiousVideoInfo struct {
	Title           string `json:"title"`
	Author          string `json:"author"`
	LengthSeconds   int    `json:"lengthSeconds"`
	AdaptiveFormats []struct {
		URL      string `json:"url"`
		Type     string `json:"type"`
		Bitrate  string `json:"bitrate"`
		AudioQuality string `json:"audioQuality"`
	} `json:"adaptiveFormats"`
}

func searchViaInvidious(query string) ([]SearchResult, error) {
	instances := []string{
		"https://inv.nadeko.net",
		"https://invidious.jing.rocks",
		"https://invidious.privacyredirect.com",
		"https://y.com.sb",
	}

	var lastErr error
	for _, instance := range instances {
		results, err := tryInvidiousSearch(instance, query)
		if err == nil {
			return results, nil
		}
		lastErr = err
		log.Printf("搜索实例 %s 失败: %v", instance, err)
	}

	return nil, fmt.Errorf("所有实例都失败了: %v", lastErr)
}

func tryInvidiousSearch(instance, query string) ([]SearchResult, error) {
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

	results := make([]SearchResult, 0, len(invResults))
	for _, item := range invResults {
		if item.Type != "video" {
			continue
		}

		duration := formatDuration(item.LengthSeconds)
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

		if len(results) >= 10 {
			break
		}
	}

	return results, nil
}

func getAudioViaInvidious(videoID string) (*AudioInfo, error) {
	instances := []string{
		"https://inv.nadeko.net",
		"https://invidious.jing.rocks",
		"https://invidious.privacyredirect.com",
		"https://y.com.sb",
	}

	var lastErr error
	for _, instance := range instances {
		info, err := tryInvidiousAudio(instance, videoID)
		if err == nil {
			return info, nil
		}
		lastErr = err
		log.Printf("音频实例 %s 失败: %v", instance, err)
	}

	return nil, fmt.Errorf("所有实例都失败了: %v", lastErr)
}

func tryInvidiousAudio(instance, videoID string) (*AudioInfo, error) {
	videoURL := fmt.Sprintf("%s/api/v1/videos/%s", instance, videoID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(videoURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var videoInfo invidiousVideoInfo
	if err := json.NewDecoder(resp.Body).Decode(&videoInfo); err != nil {
		return nil, err
	}

	// 选择最佳音频格式
	var bestAudioURL string
	var bestBitrate int

	for _, format := range videoInfo.AdaptiveFormats {
		// 只选择音频格式
		if format.Type != "" && (format.Type[:10] == "audio/mp4;" || format.Type[:11] == "audio/webm;") {
			// 尝试解析比特率
			var bitrate int
			fmt.Sscanf(format.Bitrate, "%d", &bitrate)

			if bitrate > bestBitrate {
				bestBitrate = bitrate
				bestAudioURL = format.URL
			}
		}
	}

	if bestAudioURL == "" {
		return nil, fmt.Errorf("没有找到音频流")
	}

	return &AudioInfo{
		URL:      bestAudioURL,
		Title:    videoInfo.Title,
		Author:   videoInfo.Author,
		Duration: formatDuration(videoInfo.LengthSeconds),
	}, nil
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

func extractVideoID(videoURL string) (string, error) {
	u, err := url.Parse(videoURL)
	if err != nil {
		return "", err
	}

	if u.Host == "youtu.be" {
		return u.Path[1:], nil
	}

	query := u.Query()
	videoID := query.Get("v")
	if videoID == "" {
		return "", fmt.Errorf("无法从URL中提取视频ID")
	}

	return videoID, nil
}
