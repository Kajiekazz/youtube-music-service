from flask import Flask, request, jsonify
from flask_cors import CORS
import yt_dlp
import requests
import logging
import time

app = Flask(__name__)
CORS(app)

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Invidious 实例列表
INVIDIOUS_INSTANCES = [
    "https://inv.nadeko.net",
    "https://invidious.jing.rocks",
    "https://invidious.privacyredirect.com",
    "https://y.com.sb",
]

@app.route('/')
def home():
    return '''YouTube 音乐提取服务 (yt-dlp + Invidious)

使用方法:
1. 搜索: /search?q=关键词
2. 获取音频: /audio?id=视频ID

优先使用 yt-dlp，失败时自动切换到 Invidious
'''

@app.route('/search')
def search():
    query = request.args.get('q')
    if not query:
        return jsonify({'error': '缺少搜索关键词'}), 400

    logger.info(f'搜索请求: {query}')

    # 优先使用 yt-dlp
    try:
        results = search_ytdlp(query)
        if results:
            logger.info(f'yt-dlp 搜索成功，返回 {len(results)} 条结果')
            return jsonify(results)
    except Exception as e:
        logger.warning(f'yt-dlp 搜索失败: {str(e)}，尝试 Invidious')

    # 回退到 Invidious
    try:
        results = search_invidious(query)
        logger.info(f'Invidious 搜索成功，返回 {len(results)} 条结果')
        return jsonify(results)
    except Exception as e:
        logger.error(f'所有搜索方法都失败了: {str(e)}')
        return jsonify({'error': f'搜索失败: {str(e)}'}), 500

@app.route('/audio')
def audio():
    video_id = request.args.get('id')
    video_url = request.args.get('url')

    if not video_id and not video_url:
        return jsonify({'error': '缺少视频ID或URL'}), 400

    if video_url:
        video_id = extract_video_id(video_url)
        if not video_id:
            return jsonify({'error': '无效的YouTube URL'}), 400

    logger.info(f'获取音频: {video_id}')

    # 优先使用 yt-dlp
    try:
        result = get_audio_ytdlp(video_id)
        if result:
            logger.info(f'yt-dlp 获取成功: {result["title"]}')
            return jsonify(result)
    except Exception as e:
        logger.warning(f'yt-dlp 获取失败: {str(e)}，尝试 Invidious')

    # 回退到 Invidious
    try:
        result = get_audio_invidious(video_id)
        logger.info(f'Invidious 获取成功: {result["title"]}')
        return jsonify(result)
    except Exception as e:
        logger.error(f'所有获取方法都失败了: {str(e)}')
        return jsonify({'error': f'获取音频失败: {str(e)}'}), 500

# ========== yt-dlp 实现 ==========

def search_ytdlp(query):
    ydl_opts = {
        'quiet': True,
        'no_warnings': True,
        'extract_flat': True,
        'default_search': 'ytsearch10',
    }

    with yt_dlp.YoutubeDL(ydl_opts) as ydl:
        result = ydl.extract_info(f'ytsearch10:{query}', download=False)

        if not result or 'entries' not in result:
            return []

        results = []
        for entry in result['entries']:
            if not entry:
                continue

            duration = format_duration(entry.get('duration', 0))

            results.append({
                'id': entry.get('id', ''),
                'title': entry.get('title', ''),
                'author': entry.get('uploader', ''),
                'duration': duration,
                'thumbnail': entry.get('thumbnail', '')
            })

        return results

def get_audio_ytdlp(video_id):
    ydl_opts = {
        'quiet': True,
        'no_warnings': True,
        # 优先选择最低画质的视频（包含音频），如果没有则选择纯音频
        'format': 'worst[height<=360]/bestaudio/best',
        'extract_flat': False,
        # 添加更多选项以绕过 bot 检测
        'extractor_args': {'youtube': {'player_client': ['android', 'web']}},
        'user_agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    }

    with yt_dlp.YoutubeDL(ydl_opts) as ydl:
        info = ydl.extract_info(f'https://www.youtube.com/watch?v={video_id}', download=False)

        # 获取音频/视频 URL
        audio_url = None
        if 'url' in info:
            audio_url = info['url']
        elif 'formats' in info:
            # 优先选择有音频的最低画质视频
            for fmt in info['formats']:
                if fmt.get('acodec') != 'none':
                    audio_url = fmt.get('url')
                    break

            if not audio_url and info['formats']:
                audio_url = info['formats'][-1].get('url')

        if not audio_url:
            raise Exception('没有找到音频流')

        return {
            'url': audio_url,
            'title': info.get('title', ''),
            'author': info.get('uploader', ''),
            'duration': format_duration_long(info.get('duration', 0))
        }

# ========== Invidious 实现 ==========

def search_invidious(query):
    for instance in INVIDIOUS_INSTANCES:
        try:
            url = f"{instance}/api/v1/search?q={requests.utils.quote(query)}&type=video"
            resp = requests.get(url, timeout=10)

            if resp.status_code != 200:
                continue

            data = resp.json()
            results = []

            for item in data:
                if item.get('type') != 'video':
                    continue

                duration = format_duration(item.get('lengthSeconds', 0))
                thumbnail = ''
                if item.get('videoThumbnails'):
                    thumbnail = item['videoThumbnails'][0].get('url', '')

                results.append({
                    'id': item.get('videoId', ''),
                    'title': item.get('title', ''),
                    'author': item.get('author', ''),
                    'duration': duration,
                    'thumbnail': thumbnail
                })

                if len(results) >= 10:
                    break

            return results

        except Exception as e:
            logger.debug(f'Invidious 实例 {instance} 失败: {str(e)}')
            continue

    raise Exception('所有 Invidious 实例都失败了')

def get_audio_invidious(video_id):
    for instance in INVIDIOUS_INSTANCES:
        try:
            url = f"{instance}/api/v1/videos/{video_id}"
            resp = requests.get(url, timeout=10)

            if resp.status_code != 200:
                continue

            data = resp.json()

            # 选择最佳音频格式
            best_audio_url = None
            best_bitrate = 0

            for fmt in data.get('adaptiveFormats', []):
                if fmt.get('type', '').startswith('audio/'):
                    bitrate = int(fmt.get('bitrate', '0'))
                    if bitrate > best_bitrate:
                        best_bitrate = bitrate
                        best_audio_url = fmt.get('url')

            if not best_audio_url:
                continue

            return {
                'url': best_audio_url,
                'title': data.get('title', ''),
                'author': data.get('author', ''),
                'duration': format_duration_long(data.get('lengthSeconds', 0))
            }

        except Exception as e:
            logger.debug(f'Invidious 实例 {instance} 失败: {str(e)}')
            continue

    raise Exception('所有 Invidious 实例都失败了')

# ========== 工具函数 ==========

def format_duration(seconds):
    """格式化时长为 M:SS 或 H:MM:SS"""
    if not seconds:
        return "未知"

    # 确保是整数
    seconds = int(seconds)

    hours = seconds // 3600
    minutes = (seconds % 3600) // 60
    secs = seconds % 60

    if hours > 0:
        return f"{hours}:{minutes:02d}:{secs:02d}"
    return f"{minutes}:{secs:02d}"

def format_duration_long(seconds):
    """格式化时长为 XmYs"""
    if not seconds:
        return "未知"

    # 确保是整数
    seconds = int(seconds)

    hours = seconds // 3600
    minutes = (seconds % 3600) // 60
    secs = seconds % 60

    if hours > 0:
        return f"{hours}h{minutes}m{secs}s"
    return f"{minutes}m{secs}s"

def extract_video_id(url):
    """从 YouTube URL 提取视频 ID"""
    import re
    patterns = [
        r'(?:v=|\/)([0-9A-Za-z_-]{11}).*',
        r'(?:embed\/)([0-9A-Za-z_-]{11})',
        r'(?:watch\?v=)([0-9A-Za-z_-]{11})'
    ]

    for pattern in patterns:
        match = re.search(pattern, url)
        if match:
            return match.group(1)
    return None

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)
