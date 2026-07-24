"""Parse local materials: PDF, Word, TXT, subtitles, video/audio."""

from __future__ import annotations

import os
import re
import subprocess
import tempfile
from pathlib import Path
from typing import Any

from ..chunking import clean_text


class ParseError(RuntimeError):
    pass


SUPPORTED_EXTENSIONS = {
    ".pdf", ".txt", ".md", ".markdown", ".srt", ".vtt",
    ".docx", ".doc", ".mp4", ".mkv", ".avi", ".mov",
    ".mp3", ".wav", ".flac", ".m4a", ".ogg",
}


def clean_subtitle_text(text: str) -> str:
    """Remove timestamps and sequence numbers from SRT/VTT text."""
    lines: list[str] = []
    for raw_line in text.replace("\r\n", "\n").replace("\r", "\n").split("\n"):
        line = raw_line.strip()
        if not line or line.upper() == "WEBVTT":
            continue
        if re.fullmatch(r"\d+", line):
            continue
        if "-->" in line:
            continue
        line = re.sub(r"<[^>]+>", "", line).strip()
        if line:
            lines.append(line)
    return clean_text("\n".join(lines))


def read_pdf(path: Path) -> str:
    try:
        from pypdf import PdfReader
    except ImportError:
        raise ParseError("缺少 pypdf 依赖。")

    reader = PdfReader(str(path))
    pages: list[str] = []
    for idx, page in enumerate(reader.pages, start=1):
        text = page.extract_text() or ""
        if text.strip():
            pages.append(f"\n\n<!-- page {idx} -->\n\n{text}")
    return clean_text("\n".join(pages))


def read_docx(path: Path) -> str:
    try:
        from docx import Document
    except ImportError:
        raise ParseError("缺少 python-docx 依赖。")

    doc = Document(str(path))
    paragraphs: list[str] = []
    for para in doc.paragraphs:
        if para.text.strip():
            paragraphs.append(para.text)
    return clean_text("\n".join(paragraphs))


def read_txt(path: Path) -> str:
    text = path.read_text(encoding="utf-8")
    suffix = path.suffix.lower()
    if suffix in {".srt", ".vtt"}:
        return clean_subtitle_text(text)
    return clean_text(text)


def extract_audio(video_path: Path, audio_path: Path) -> None:
    """Extract audio track from video using FFmpeg."""
    cmd = [
        "ffmpeg", "-i", str(video_path),
        "-vn", "-acodec", "pcm_s16le",
        "-ar", "16000", "-ac", "1",
        str(audio_path), "-y",
    ]
    try:
        subprocess.run(cmd, check=True, capture_output=True, text=True)
    except subprocess.CalledProcessError as e:
        raise ParseError(f"FFmpeg 提取音频失败：{e.stderr}")
    except FileNotFoundError:
        raise ParseError("请先安装 FFmpeg 并配置系统环境变量。")


def audio_to_text(audio_path: Path) -> str:
    """Convert audio to text using SpeechRecognition."""
    try:
        import speech_recognition as sr
    except ImportError:
        raise ParseError("缺少 SpeechRecognition 依赖。")

    recognizer = sr.Recognizer()
    try:
        with sr.AudioFile(str(audio_path)) as source:
            audio = recognizer.record(source)
        return recognizer.recognize_google(audio, language="zh-CN")
    except Exception as exc:
        raise ParseError(f"语音识别失败：{exc}")


def process_media(path: Path, convert_audio: bool = True) -> str:
    """Extract text from video/audio file.
    For video: tries embedded subtitles first, then extracts audio → speech-to-text.
    For audio: direct speech-to-text.
    """
    suffix = path.suffix.lower()
    is_video = suffix in {".mp4", ".mkv", ".avi", ".mov"}
    is_audio = suffix in {".mp3", ".wav", ".flac", ".m4a", ".ogg"}

    if not is_video and not is_audio:
        raise ParseError(f"不支持的媒体格式：{suffix}")

    if not convert_audio:
        return f"[媒体文件已记录，跳过音频转换]：{path}"

    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
        tmp_path = Path(tmp.name)

    try:
        if is_video:
            extract_audio(path, tmp_path)
        else:
            # Already audio, copy/use directly
            import shutil
            shutil.copy(path, tmp_path)

        text = audio_to_text(tmp_path)
        return clean_text(text)
    finally:
        if tmp_path.exists():
            tmp_path.unlink(missing_ok=True)


def parse_local_file(path: Path | str, convert_audio: bool = True) -> str:
    """Parse a local file and return its text content.

    Args:
        path: Path to the file.
        convert_audio: Whether to run speech-to-text on media files.

    Returns:
        Extracted text content.
    """
    path = Path(path)
    if not path.exists():
        raise ParseError(f"文件不存在：{path}")

    suffix = path.suffix.lower()
    if suffix not in SUPPORTED_EXTENSIONS:
        raise ParseError(f"暂不支持文件类型 {suffix}。支持：{[e for e in sorted(SUPPORTED_EXTENSIONS)]}")

    if suffix == ".pdf":
        return read_pdf(path)
    elif suffix in {".docx", ".doc"}:
        return read_docx(path)
    elif suffix in {".txt", ".md", ".markdown", ".srt", ".vtt"}:
        return read_txt(path)
    elif suffix in {".mp4", ".mkv", ".avi", ".mov", ".mp3", ".wav", ".flac", ".m4a", ".ogg"}:
        return process_media(path, convert_audio=convert_audio)
    else:
        raise ParseError(f"未处理的文件类型：{suffix}")
