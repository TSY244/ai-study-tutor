"""Run knowledge extraction via local claude CLI (non-interactive, --model required).

Replaces the Docker-based approach.  Calls `claude -p` directly on the host.
"""

from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path


class ExtractionError(RuntimeError):
    """Raised when claude CLI extraction fails."""


# ---------------------------------------------------------------------------
# Prompt template
# ---------------------------------------------------------------------------

def build_extraction_prompt(
    source_description: str,
    subject: str,
    lesson: int,
    name: str,
) -> str:
    """Build the prompt that claude CLI will execute."""
    return f"""你需要完成以下知识提取任务，所有输出文件写入 /output/ 目录。

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
如果没有字幕，使用 WebFetch + WebSearch 获取视频信息和第三方分析。

### 步骤 2：生成教师教纲（teacher_notes）
整理全部文本内容，生成教师教纲（Markdown），写入 /output/teacher_notes/content.md

教纲结构：
# 教师教纲：{name}
## 课程信息
## 教学目标（3-5条具体目标）
## 知识点体系（层级列表，标注重难点）
## 教学流程（导入→核心讲解→练习→小结，每段标注时长）
## 板书设计
## 课后作业建议
## 教学反思要点

### 步骤 3：生成学生学习笔记（student_notes）
基于教纲生成学生笔记（Markdown），写入 /output/student_notes/content.md

笔记结构：
# {name} — 学习笔记
## 本课目标
## 前置知识
## 核心知识点（定义、原理、举例、易错点）
## 重点标注（> ⚠️ 标注）
## 常见误区
## 记忆技巧（口诀、类比、图表）
## 自测清单（- [ ] ...）
## 课后练习建议

### 步骤 4：生成课程元信息
写入 /output/info.json：
{{"subject": "{subject}", "lesson": {lesson}, "name": "{name}"}}

## 重要规则
- 所有文件用 Write 写入 /output/ 目录
- 内容只基于素材，不编造
- 中文表达清晰，适合初学者
- 完成后输出 EXTRACTION_COMPLETE"""


# ---------------------------------------------------------------------------
# Claude CLI runner
# ---------------------------------------------------------------------------

def _load_claude_settings() -> dict[str, str]:
    """Load env vars from ~/.claude/settings.json."""
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
    model: str,
    api_key: str,
    base_url: str | None = None,
    timeout: int = 900,
) -> None:
    """Run knowledge extraction via local claude CLI.

    Args:
        prompt: Full prompt for claude to execute.
        output_dir: Directory for output files (claude sees this as /output via symlink/mount? No — we map it).
        model: Model name (required, passed as --model).
        api_key: API key.
        base_url: Optional custom base URL.
        timeout: Max seconds.
    """
    if not model:
        raise ExtractionError("必须指定 --model 参数。")

    output_dir.mkdir(parents=True, exist_ok=True)

    # Load Claude Code settings for auth
    claude_env = _load_claude_settings()

    # Build env
    env = os.environ.copy()
    env["ANTHROPIC_AUTH_TOKEN"] = claude_env.get("ANTHROPIC_AUTH_TOKEN") or api_key

    # Use base_url from claude settings preferentially
    effective_base_url = base_url
    if not effective_base_url:
        effective_base_url = claude_env.get("ANTHROPIC_BASE_URL")

    if effective_base_url:
        env["ANTHROPIC_BASE_URL"] = effective_base_url

    # Strip context-window suffix from model name
    clean_model = re.sub(r"\[\d+m\]$", "", model)

    # Build claude command
    cmd = [
        "claude",
        "--model", clean_model,
        "-p", prompt,
        "--allowedTools", "Bash,Read,Write,WebFetch,WebSearch",
    ]

    # Replace /output/ in prompt with actual output directory path
    actual_output = str(output_dir.resolve())
    prompt = prompt.replace("/output/", actual_output + "/")

    # Ensure output subdirs exist
    (output_dir / "teacher_notes").mkdir(parents=True, exist_ok=True)
    (output_dir / "student_notes").mkdir(parents=True, exist_ok=True)

    print(f"🤖 启动 claude --model {clean_model} 进行知识提取...")
    print(f"   输出目录：{output_dir}")
    print()

    try:
        proc = subprocess.Popen(
            cmd,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            cwd=str(output_dir),
        )
        assert proc.stdout is not None

        for line in proc.stdout:
            sys.stdout.write(line)
            sys.stdout.flush()

        proc.wait(timeout=timeout)

        if proc.returncode != 0:
            raise ExtractionError(
                f"claude CLI 执行失败（退出码 {proc.returncode}）。"
            )

    except subprocess.TimeoutExpired:
        proc.kill()
        raise ExtractionError(f"知识提取超时（{timeout}秒）。")

    # Fix up: claude sometimes writes to output/ subdirectory
    normalize_output(output_dir)


def verify_output(output_dir: Path) -> dict[str, bool]:
    """Check which expected output files exist (checks both root and output/ subdir)."""
    # Claude sometimes writes to output/ subdirectory instead of root
    candidates = [output_dir, output_dir / "output"]

    def _find(rel_path: str) -> bool:
        for base in candidates:
            if (base / rel_path).exists():
                return True
        return False

    return {
        "teacher_notes": _find("teacher_notes/content.md"),
        "student_notes": _find("student_notes/content.md"),
        "info_json": _find("info.json"),
    }


def normalize_output(output_dir: Path) -> None:
    """If claude wrote files to output_dir/output/, move them up to output_dir/."""
    nested = output_dir / "output"
    if not nested.exists() or not nested.is_dir():
        return
    import shutil
    for item in nested.iterdir():
        dest = output_dir / item.name
        if dest.exists():
            if dest.is_dir():
                shutil.rmtree(dest)
            else:
                dest.unlink()
        shutil.move(str(item), str(dest))
    # Remove the now-empty output/ dir
    try:
        nested.rmdir()
    except OSError:
        pass
