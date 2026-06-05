"""Backend service layer — all business logic, no user I/O.

The frontend (interactive.py / main.py CLI) calls these functions.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

from .config_loader import get_config
from .storage import Storage, StorageError


# ── Lazy singleton ──

_svc = None

def svc() -> "CourseService":
    global _svc
    if _svc is None:
        _svc = CourseService()
    return _svc


class CourseService:
    """Stateless service that wraps Storage + extraction + exam logic."""

    def __init__(self) -> None:
        cfg = get_config()
        self.storage = Storage(cfg.course_path)
        self.cfg = cfg

    # ── Subject CRUD ──

    def list_subjects(self) -> list[str]:
        return self.storage.list_subjects()

    def create_subject(self, name: str) -> Path:
        if not name.strip():
            raise ValueError("科目名称不能为空。")
        return self.storage.create_subject(name.strip())

    def delete_subject(self, name: str) -> bool:
        return self.storage.delete_subject(name)

    # ── Lesson CRUD ──

    def list_lessons(self, subject: str) -> list[int]:
        return sorted(self.storage.list_lessons(subject))

    def lesson_info(self, subject: str, lesson: int) -> dict[str, Any] | None:
        p = self.storage.info_path(subject, lesson)
        if p.exists():
            return self.storage.read_json(p)
        return None

    def lesson_name(self, subject: str, lesson: int) -> str:
        info = self.lesson_info(subject, lesson)
        if info:
            return info.get("name", f"第{lesson}课")
        return f"第{lesson}课"

    def lesson_status(self, subject: str, lesson: int) -> dict[str, bool]:
        """Return {teacher, student, quiz} — whether each exists."""
        s = self.storage
        return {
            "teacher": s.teacher_notes_path(subject, lesson).exists(),
            "student": s.student_notes_path(subject, lesson).exists(),
            "quiz": bool(list(s.exam_dir(subject, lesson).glob("quiz_chapter_*.json"))),
        }

    def delete_lesson(self, subject: str, lesson: int) -> bool:
        return self.storage.delete_lesson(subject, lesson)

    # ── Import ──

    def import_lesson(
        self,
        subject: str,
        lesson: int,
        name: str,
        source_flag: str,  # "--load-url" / "--load-video" / "--source-text"
        source_value: str,
        model: str = "deepseek-v4-pro",
    ) -> int:
        """Run claude extraction. Returns exit code."""
        import subprocess, sys
        from pathlib import Path

        project_root = Path(__file__).resolve().parent.parent
        # Ensure output dirs exist
        lesson_dir = self.storage.ensure_lesson_dir(subject, lesson)
        (lesson_dir / "teacher_notes").mkdir(parents=True, exist_ok=True)
        (lesson_dir / "student_notes").mkdir(parents=True, exist_ok=True)

        # Load auth from Claude Code settings
        auth_token = ""
        base_url = ""
        claude_settings = Path.home() / ".claude" / "settings.json"
        if claude_settings.exists():
            import json
            env_data = json.loads(claude_settings.read_text()).get("env", {})
            auth_token = env_data.get("ANTHROPIC_AUTH_TOKEN", "")
            base_url = env_data.get("ANTHROPIC_BASE_URL", "")

        # Fall back to config.json
        if not auth_token:
            provider_cfg = self.cfg.provider_config()
            auth_token = provider_cfg.get("api_key", "")
        if not base_url:
            provider_cfg = self.cfg.provider_config()
            base_url = provider_cfg.get("base_url", "")

        # Build prompt
        from .extraction_runner import build_extraction_prompt
        source_desc = f"素材：{source_value}"
        if source_flag == "--load-url":
            source_desc = (
                f"从视频链接提取知识：{source_value}\n"
                "先用 yt-dlp 获取字幕。无字幕则用 WebFetch + WebSearch。"
            )
        elif source_flag == "--load-video":
            source_desc = f"从本地文件提取知识：{source_value}"
        elif source_flag == "--source-text":
            raw_path = self.storage.raw_text_path(subject, lesson)
            self.storage.save_text(raw_path, source_value)
            source_desc = f"文本已保存在 {raw_path}，请 Read 后生成笔记。"

        prompt = build_extraction_prompt(source_desc, subject, lesson, name)
        # Replace /output/ with actual path
        prompt = prompt.replace("/output/", str(lesson_dir) + "/")

        env = __import__("os").environ.copy()
        env["ANTHROPIC_AUTH_TOKEN"] = auth_token
        if base_url:
            env["ANTHROPIC_BASE_URL"] = base_url

        clean_model = __import__("re").sub(r"\[\d+m\]$", "", model)

        cmd = [
            "claude", "--model", clean_model,
            "-p", prompt,
            "--allowedTools", "Bash,Read,Write,WebFetch,WebSearch",
        ]

        proc = subprocess.Popen(cmd, env=env, cwd=str(lesson_dir))
        proc.wait()
        return proc.returncode

    def verify_import(self, subject: str, lesson: int) -> dict[str, bool]:
        from .extraction_runner import verify_output, normalize_output
        lesson_dir = self.storage.lesson_dir(subject, lesson)
        normalize_output(lesson_dir)
        return verify_output(lesson_dir)

    # ── Exam ──

    def generate_exam(self, subject: str, lesson: int, num: int, model: str) -> int:
        import subprocess, sys
        project_root = Path(__file__).resolve().parent.parent
        proc = subprocess.Popen([
            sys.executable, str(project_root / "main.py"),
            "--exam-new", "--subject", subject, "--lesson", str(lesson),
            "--num", str(num), "--model", model,
        ])
        proc.wait()
        return proc.returncode

    # ── Wrong questions ──

    def wrong_count(self, subject: str, lesson: int) -> int:
        try:
            from .exam_logic import list_wrong_questions
            return len(list_wrong_questions(subject, lesson, self.storage))
        except Exception:
            return 0
