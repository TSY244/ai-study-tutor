"""Run knowledge extraction inside an isolated Docker container via Claude Code CLI.

Flow:
  1. Build (if needed) or use the ai-tutor-runner Docker image
  2. Start a container with:
     - API credentials as env vars
     - Output directory mounted for file extraction
  3. Run `claude -p "<prompt>"` inside the container
  4. Wait for completion
  5. Generated files are already on the host via volume mount
  6. Remove the container
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any


class ContainerError(RuntimeError):
    """Raised when Docker container operations fail."""


IMAGE_NAME = "ai-tutor-runner"

# ---------------------------------------------------------------------------
# Prompt templates for knowledge extraction
# ---------------------------------------------------------------------------

def build_extraction_prompt(
    source_description: str,
    subject: str,
    lesson: int,
    name: str,
) -> str:
    """Build the prompt that Claude Code will execute inside the container.

    Claude Code has access to Bash (yt-dlp, ffmpeg), Read, Write, WebFetch.
    It will download subtitles/metadata, analyze the content, and write
    the output files directly to the mounted output directory.
    """
    prompt = f"""你需要完成以下知识提取任务，所有输出文件写入 /output/ 目录。

## 任务信息
- 科目：{subject}
- 课时编号：{lesson}
- 课时名称：{name}
- 素材来源：{source_description}

## 执行步骤

### 步骤 1：获取素材文本
{source_description}

如果是 YouTube/B站 URL，用 yt-dlp 下载字幕（中文字幕优先）：
```bash
yt-dlp --write-subs --write-auto-subs --sub-langs "zh-Hans,zh,en,zh-CN" --skip-download --convert-subs srt -o "/output/raw_src" "{source_description.strip()}"
```
如果下载失败，尝试只获取元数据：
```bash
yt-dlp --dump-json "{source_description.strip()}" > /output/video_info.json
```

如果下载成功，读取字幕文件内容汇总全文。

如果是本地文件路径，直接 Read 读取文件内容。

### 步骤 2：生成教师教纲（teacher_notes）
将全部文本内容整理后，生成一份详细的教师教纲（Markdown），写入：
```
/output/teacher_notes/content.md
```

教纲结构要求：
```markdown
# 教师教纲：{name}
## 课程信息
- 科目：{subject}
- 课时：第{lesson}课：{name}
- 素材来源：{source_description.strip()}

## 教学目标
（基于素材内容提炼 3-5 条具体教学目标）

## 知识点体系
（用层级列表或思维导图方式列出全部知识点，标注重难点）

## 教学流程
### 导入（5分钟）
### 核心讲解（按知识点分段，每段标注时长）
### 练习巩固
### 课堂小结

## 板书设计
（关键公式、图表、框架）

## 课后作业建议
## 教学反思要点
```

### 步骤 3：生成学生学习笔记（student_notes）
基于教纲，生成一份适合学生自学的详细笔记（Markdown），写入：
```
/output/student_notes/content.md
```

笔记结构要求：
```markdown
# {name} — 学习笔记
## 本课目标
（学完本课你应该掌握什么）

## 前置知识
（需要先掌握哪些基础）

## 核心知识点
（每个知识点包括：定义、原理、举例、易错点）

## 重点标注
（用 > ⚠️ 标注特别重要的内容）

## 常见误区
（列举初学者容易犯的错误）

## 记忆技巧
（口诀、类比、图表等帮助记忆的方法）

## 自测清单
- [ ] 是否能...
- [ ] 是否能...

## 课后练习建议
```

### 步骤 4：生成课程元信息
写入 /output/info.json：
```json
{{
  "subject": "{subject}",
  "lesson": {lesson},
  "name": "{name}",
  "source_description": "{source_description.strip()[:200]}"
}}
```

## 重要规则
- 所有文件必须用 Write 工具写入 /output/ 目录
- 内容只基于素材，不要编造
- 中文表达清晰，适合初学者
- 完成后输出 "EXTRACTION_COMPLETE" 表示任务结束
"""
    return prompt


def build_local_file_prompt(
    file_paths: list[str],
    subject: str,
    lesson: int,
    name: str,
) -> str:
    """Build prompt for local file processing (video/audio/doc)."""
    files_list = "\n".join(f"  - {p}" for p in file_paths)
    source_desc = f"本地文件：\n{files_list}\n\n"
    source_desc += "对于视频/音频文件，使用 ffmpeg 和 yt-dlp 提取音频或字幕。对于文档，直接 Read 读取。"

    return build_extraction_prompt(source_desc, subject, lesson, name)


# ---------------------------------------------------------------------------
# Docker container lifecycle
# ---------------------------------------------------------------------------

def _run_docker(args: list[str], check: bool = True, timeout: int = 600) -> subprocess.CompletedProcess:
    """Run a docker command, raise ContainerError on failure."""
    try:
        return subprocess.run(
            ["docker"] + args,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        raise ContainerError("Docker 操作超时（10分钟）。")
    except FileNotFoundError:
        raise ContainerError("未找到 Docker。请确认 Docker 已安装并在 PATH 中。")


def image_exists() -> bool:
    """Check if the ai-tutor-runner image exists."""
    result = _run_docker(["images", "-q", IMAGE_NAME], check=False)
    return bool(result.stdout.strip())


def build_image() -> None:
    """Build the Docker image for the runner."""
    dockerfile_dir = Path(__file__).resolve().parent.parent  # project root

    if not (dockerfile_dir / "Dockerfile").exists():
        raise ContainerError(f"未找到 Dockerfile：{dockerfile_dir}")

    print(f"🔨 正在构建 Docker 镜像 {IMAGE_NAME}...（首次需要几分钟）")
    result = _run_docker(
        ["build", "-t", IMAGE_NAME, str(dockerfile_dir)],
        timeout=1200,  # 20 min for first build
        check=False,
    )
    if result.returncode != 0:
        raise ContainerError(f"Docker 镜像构建失败：\n{result.stderr[-500:]}")
    print(f"✅ 镜像构建完成：{IMAGE_NAME}")


def _load_claude_settings() -> dict[str, str]:
    """Load Claude Code settings from ~/.claude/settings.json for env vars."""
    settings_path = Path.home() / ".claude" / "settings.json"
    if not settings_path.exists():
        return {}
    try:
        import json
        data = json.loads(settings_path.read_text(encoding="utf-8"))
        return data.get("env", {})
    except Exception:
        return {}


def run_extraction(
    prompt: str,
    output_dir: Path,
    api_key: str,
    base_url: str | None = None,
    model: str | None = None,
    timeout: int = 600,
) -> None:
    """Run knowledge extraction inside a Docker container.

    Credentials are merged from:
      1. ~/.claude/settings.json (ANTHROPIC_BASE_URL, ANTHROPIC_MODEL, etc.)
      2. config.json (api_key → ANTHROPIC_AUTH_TOKEN)
      3. Function arguments (override)

    Args:
        prompt: The full prompt for Claude Code to execute.
        output_dir: Host directory to mount for output files.
        api_key: API key (set as ANTHROPIC_AUTH_TOKEN).
        base_url: Optional custom Anthropic-compatible base URL.
        model: Optional model name.
        timeout: Max seconds to wait for container to finish.
    """
    # Ensure image exists
    if not image_exists():
        build_image()

    # Ensure output directory exists
    output_dir.mkdir(parents=True, exist_ok=True)

    # Load user's Claude Code settings for env vars
    claude_env = _load_claude_settings()

    # Build env vars for the container (merge: claude settings < function args)
    # Prefer ANTHROPIC_AUTH_TOKEN from Claude Code settings over config.json api_key,
    # since the Claude Code CLI inside the container uses the same auth as the host.
    auth_token = claude_env.get("ANTHROPIC_AUTH_TOKEN") or api_key
    container_env: dict[str, str] = {
        "ANTHROPIC_AUTH_TOKEN": auth_token,
    }

    # Copy relevant env vars from Claude Code settings
    for key in (
        "ANTHROPIC_BASE_URL",
        "ANTHROPIC_MODEL",
        "ANTHROPIC_DEFAULT_OPUS_MODEL",
        "ANTHROPIC_DEFAULT_SONNET_MODEL",
        "ANTHROPIC_DEFAULT_HAIKU_MODEL",
        "ANTHROPIC_REASONING_MODEL",
    ):
        if key in claude_env:
            val = claude_env[key]
            # Strip Claude Code context-window suffix (e.g. "[1m]") for non-Anthropic backends
            import re as _re
            val = _re.sub(r"\[\d+m\]$", "", val)
            # Skip ANTHROPIC_REASONING_MODEL as some backends reject it
            if key == "ANTHROPIC_REASONING_MODEL":
                continue
            container_env[key] = val

    # Apply explicit overrides only if Claude Code settings don't already have them.
    # config.json has OpenAI-compatible base URLs, but Claude Code needs Anthropic-format
    # endpoints.  Prefer the Claude Code settings values.
    if base_url and "ANTHROPIC_BASE_URL" not in container_env:
        container_env["ANTHROPIC_BASE_URL"] = base_url
    if model and "ANTHROPIC_MODEL" not in container_env:
        import re as _re2
        container_env["ANTHROPIC_MODEL"] = _re2.sub(r"\[\d+m\]$", "", model)

    # Build docker run command
    docker_args = [
        "run",
        "--rm",  # Auto-remove after exit
        "--name", f"ai-tutor-{os.getpid()}-{abs(hash(prompt)) % 10000}",
    ]

    # Add all env vars
    for key, value in container_env.items():
        docker_args.extend(["-e", f"{key}={value}"])

    # Mount output directory
    docker_args.extend(["-v", f"{output_dir.resolve()}:/output"])

    # Determine which model to use
    effective_model = container_env.get("ANTHROPIC_MODEL", "claude-sonnet-4-6")

    # Image and claude command
    docker_args.append(IMAGE_NAME)
    docker_args.extend([
        "claude",
        "--model", effective_model,
        "-p", prompt,  # Print (non-interactive) mode
        "--allowedTools", "Bash,Read,Write,WebFetch,WebSearch",
    ])

    print(f"🐳 启动隔离容器执行知识提取...")
    print(f"   输出目录：{output_dir}")
    print(f"   模型：{model or '默认'}")
    print()

    # Stream output so user sees progress
    try:
        proc = subprocess.Popen(
            ["docker"] + docker_args,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        assert proc.stdout is not None

        for line in proc.stdout:
            # Pass through claude's output for visibility
            sys.stdout.write(line)
            sys.stdout.flush()

        proc.wait(timeout=timeout)

        if proc.returncode != 0:
            raise ContainerError(
                f"容器内 Claude Code 执行失败（退出码 {proc.returncode}）。"
            )

    except subprocess.TimeoutExpired:
        proc.kill()
        # Force remove the container
        container_name = docker_args[3]  # --name value
        _run_docker(["rm", "-f", container_name], check=False)
        raise ContainerError(f"知识提取超时（{timeout}秒）。请检查网络或素材大小。")


def verify_output(output_dir: Path) -> dict[str, bool]:
    """Check which expected output files exist."""
    return {
        "teacher_notes": (output_dir / "teacher_notes" / "content.md").exists(),
        "student_notes": (output_dir / "student_notes" / "content.md").exists(),
        "info_json": (output_dir / "info.json").exists(),
        "raw_src": (
            (output_dir / "raw_src").exists() or
            bool(list(output_dir.glob("raw_src*"))) or
            (output_dir / "video_info.json").exists()
        ),
    }
