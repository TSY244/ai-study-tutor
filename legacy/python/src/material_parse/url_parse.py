"""Parse online materials: B站 (Bilibili) and YouTube videos via yt-dlp."""

from __future__ import annotations

import os
import re
from pathlib import Path

from ..chunking import clean_text


class URLParseError(RuntimeError):
    pass


def is_bilibili(url: str) -> bool:
    return "bilibili.com" in url.lower()


def is_youtube(url: str) -> bool:
    return "youtube.com" in url.lower() or "youtu.be" in url.lower()


def _clean_vtt(text: str) -> str:
    """Basic VTT cleaning — remove timestamps and WEBVTT header."""
    lines: list[str] = []
    for line in text.replace("\r\n", "\n").replace("\r", "\n").split("\n"):
        line = line.strip()
        if not line or line.upper() == "WEBVTT" or line.upper().startswith("NOTE"):
            continue
        if re.fullmatch(r"\d+", line):
            continue
        if "-->" in line:
            continue
        line = re.sub(r"<[^>]+>", "", line).strip()
        line = re.sub(r"</?c[^>]*>", "", line)
        if line:
            lines.append(line)
    return clean_text("\n".join(lines))


def _find_cookies_file() -> str | None:
    """Locate a browser cookies file that yt-dlp can use."""
    candidates = [
        # Chrome
        os.path.expanduser("~/Library/Application Support/Google/Chrome/Default/Cookies"),
        # Firefox
        os.path.expanduser("~/Library/Application Support/Firefox/Profiles"),
        # Netscape-format cookies.txt
        Path("cookies.txt"),
        os.path.expanduser("~/cookies.txt"),
    ]
    for p in candidates:
        path = Path(p)
        if path.exists() and path.is_file():
            return str(path)
    return None


def parse_url(
    url: str,
    save_dir: Path | str,
    download_sub_only: bool = True,
    cookies_file: str | None = None,
) -> str:
    """Download subtitles from B站/YouTube and return text.

    Args:
        url: Video URL (Bilibili or YouTube).
        save_dir: Directory to save downloaded files.
        download_sub_only: If True, only download subtitles (no video).
        cookies_file: Path to browser cookies file for sites that require login.

    Returns:
        Extracted subtitle/transcript text.
    """
    try:
        import yt_dlp
    except ImportError:
        raise URLParseError("缺少 yt-dlp 依赖。请运行：pip install yt-dlp")

    save_path = Path(save_dir)
    save_path.mkdir(parents=True, exist_ok=True)

    outtmpl = str(save_path / "src")

    ydl_opts: dict = {
        "outtmpl": outtmpl,
        "writesubtitles": True,
        "writeautomaticsub": True,
        "subtitleslangs": ["zh-Hans", "zh", "en", "zh-CN", "zh-TW", "zh-Hant", "ai-zh"],
        "skip_download": download_sub_only,
        "quiet": True,
        "no_warnings": True,
        "socket_timeout": 30,
        "retries": 3,
    }

    # B站 specific: try cookies first, fall back to custom headers
    if is_bilibili(url):
        if cookies_file and Path(cookies_file).exists():
            ydl_opts["cookiefile"] = cookies_file
        else:
            # Auto-detect browser cookies
            auto_cookies = _find_cookies_file()
            if auto_cookies:
                ydl_opts["cookiefile"] = auto_cookies

        ydl_opts["http_headers"] = {
            "Referer": "https://www.bilibili.com/",
            "User-Agent": (
                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                "AppleWebKit/537.36 (KHTML, like Gecko) "
                "Chrome/131.0.0.0 Safari/537.36"
            ),
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
        }

    # Attempt download
    try:
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            ydl.extract_info(url, download=True)
    except Exception as exc:
        msg = str(exc)
        if "412" in msg or "Precondition Failed" in msg:
            raise URLParseError(
                "B站拒绝访问（HTTP 412 反爬虫拦截）。\n\n"
                "解决方案：\n"
                "1. 导出浏览器 cookies 为 cookies.txt 放到项目根目录：\n"
                "   Chrome 插件: Get cookies.txt LOCALLY\n"
                "   或使用: yt-dlp --cookies-from-browser chrome\n"
                "2. 也可以先下载视频，再用 --load-video 导入：\n"
                "   python main.py --load-video video.mp4 --subject \"K线\" --lesson 1 --name \"技术分析\"\n"
                "3. 如果视频有人工字幕，可以手动下载字幕文件(.srt/.vtt)，用 --load-doc 导入"
            )
        raise URLParseError(f"视频下载/解析失败：{exc}")

    # Find the downloaded subtitle file
    text = _find_subtitle_text(save_path)

    if not text.strip():
        # Check if an info.json was saved (yt-dlp metadata)
        info_files = list(save_path.glob("*.info.json"))
        if info_files:
            import json
            info = json.loads(info_files[0].read_text(encoding="utf-8"))
            subs = info.get("subtitles", {}) or info.get("automatic_captions", {})
            available = list(subs.keys())
            msg = "未能获取字幕文本。"
            if available:
                msg += f"\n可用的字幕语言：{available}"
                msg += "\n请尝试设置 yt-dlp 的 subtitleslangs 参数。"
            raise URLParseError(msg)

        raise URLParseError(
            "未能获取字幕。可能该视频没有字幕。\n\n"
            "解决方案：\n"
            "1. 检查视频是否有人工/AI 字幕\n"
            "2. 如果视频在 YouTube 上，YouTube 通常有自动生成字幕\n"
            "3. 使用 --load-video 下载视频后，通过音频转文字导入：\n"
            "   python main.py --load-video video.mp4 --subject \"科目\" --lesson 1 --name \"名称\""
        )

    # Save the cleaned text
    raw_text_path = save_path / "raw_text.md"
    raw_text_path.write_text(text, encoding="utf-8")
    return text


def _find_subtitle_text(save_path: Path) -> str:
    """Search for downloaded subtitle files and return cleaned text."""
    subtitle_extensions = [".vtt", ".srt", ".txt", ".ass", ".ssa", ".lrc"]
    text = ""
    for ext in subtitle_extensions:
        candidates = list(save_path.glob(f"*{ext}"))
        candidates += list(save_path.glob(f"src*{ext}"))
        for candidate in candidates:
            try:
                raw = candidate.read_text(encoding="utf-8")
            except UnicodeDecodeError:
                continue
            if ext == ".vtt":
                text = _clean_vtt(raw)
            elif ext == ".srt":
                from .local_parse import clean_subtitle_text
                text = clean_subtitle_text(raw)
            elif ext == ".lrc":
                text = _clean_lrc(raw)
            else:
                text = clean_text(raw)
            if text.strip():
                return text
    return ""


def _clean_lrc(text: str) -> str:
    """Clean LRC lyric/subtitle format."""
    lines = []
    for line in text.splitlines():
        # Remove timestamps like [00:01.23]
        cleaned = re.sub(r"\[\d{2}:\d{2}[.\]]\d*\]", "", line).strip()
        if cleaned:
            lines.append(cleaned)
    return "\n".join(lines)
