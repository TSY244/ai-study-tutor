from __future__ import annotations

from pathlib import Path

import pytest

from src import exam_logic
from src import interactive
from src.config_loader import CLAUDE_AGENT_COMMAND, CODEX_AGENT_COMMAND
from src.models import CognitiveLevel, QuestionType, Quiz, QuizQuestion
from src.storage import Storage


def test_ask_returns_none_for_back_command(monkeypatch):
    monkeypatch.setattr("builtins.input", lambda _prompt: "0")

    assert interactive.ask("课时编号", allow_back=True) is None


def test_ask_agent_command_uses_config_default(monkeypatch, capsys):
    class FakeConfig:
        agent_command = CODEX_AGENT_COMMAND

    prompts: list[str] = []

    monkeypatch.setattr(interactive, "get_config", lambda: FakeConfig())
    monkeypatch.setattr("builtins.input", lambda prompt: prompts.append(prompt) or "")

    assert interactive.ask_agent_command() == CODEX_AGENT_COMMAND
    assert "AI Agent" in prompts[0]
    out = capsys.readouterr().out
    assert "Codex" in out
    assert "Claude" in out


def test_ask_agent_command_can_choose_claude_preset(monkeypatch):
    monkeypatch.setattr("builtins.input", lambda _prompt: "2")

    assert interactive.ask_agent_command() == CLAUDE_AGENT_COMMAND


def test_ask_agent_command_accepts_custom_command(monkeypatch):
    custom = "custom-agent --non-interactive"
    monkeypatch.setattr("builtins.input", lambda _prompt: custom)

    assert interactive.ask_agent_command() == custom


def test_view_student_notes_shows_original_markdown_path(
    tmp_path: Path, capsys, monkeypatch
):
    storage = Storage(tmp_path / "course")
    storage.ensure_lesson_dir("高数", 1)
    notes_path = storage.student_notes_path("高数", 1)
    storage.save_text(notes_path, "# 学生笔记\n\n这里是内容。")

    class FakeService:
        def __init__(self, storage: Storage) -> None:
            self.storage = storage

        def lesson_name(self, _subject: str, _lesson: int) -> str:
            return "极限基础"

    monkeypatch.setattr(interactive, "clear", lambda: None)
    monkeypatch.setattr(interactive, "wait", lambda: None)

    interactive._view_student_notes(FakeService(storage), "高数", 1)

    out = capsys.readouterr().out
    assert "# 学生笔记" in out
    assert "原始文件" in out
    assert str(notes_path.resolve()) in out


def test_run_cli_quiz_back_command_aborts_without_saving(tmp_path: Path, monkeypatch):
    storage = Storage(tmp_path / "course")
    quiz = Quiz(
        id="quiz_chapter_01",
        course_id="高数",
        chapter_id=1,
        title="第 1 课测试",
        questions=[
            QuizQuestion(
                id="q1",
                type=QuestionType.single_choice,
                question="极限的记号是什么？",
                options={"A": "lim", "B": "sum"},
                correct_answer="A",
                explanation="lim 表示极限。",
                tested_concepts=["极限"],
                cognitive_level=CognitiveLevel.recall,
            )
        ],
    )
    monkeypatch.setattr("builtins.input", lambda _prompt: "0")

    with pytest.raises(RuntimeError, match="退出答题"):
        exam_logic.run_cli_quiz(storage, "高数", quiz)

    assert list(storage.exam_dir("高数", 1).glob("*_answers.json")) == []


def test_run_cli_quiz_accepts_b_as_choice_answer(tmp_path: Path, monkeypatch):
    storage = Storage(tmp_path / "course")
    quiz = Quiz(
        id="quiz_chapter_01",
        course_id="基金",
        chapter_id=1,
        title="第 1 课测试",
        questions=[
            QuizQuestion(
                id="q1",
                type=QuestionType.single_choice,
                question="基金托管人通常是谁？",
                options={"A": "基金管理人", "B": "商业银行", "C": "销售平台"},
                correct_answer="B",
                explanation="基金托管人通常由商业银行担任。",
                tested_concepts=["基金托管人"],
                cognitive_level=CognitiveLevel.recall,
            )
        ],
    )
    monkeypatch.setattr("builtins.input", lambda _prompt: "B")

    _attempt_path, auto_score, wrong_answers = exam_logic.run_cli_quiz(storage, "基金", quiz)

    assert auto_score == 5
    assert wrong_answers == []
    assert len(list(storage.exam_dir("基金", 1).glob("*_answers.json"))) == 1


def test_exam_wrong_generation_uses_configured_command_without_prompt(monkeypatch):
    class FakeService:
        def wrong_count(self, _subject: str, _lesson: int) -> int:
            return 1

        def lesson_name(self, _subject: str, _lesson: int) -> str:
            return "金字塔定投法"

    captured: dict[str, list[str]] = {}

    class FakePopen:
        def __init__(self, cmd: list[str]) -> None:
            captured["cmd"] = cmd

        def wait(self) -> int:
            return 0

    monkeypatch.setattr(interactive, "clear", lambda: None)
    monkeypatch.setattr(interactive, "wait", lambda: None)
    monkeypatch.setattr("builtins.input", lambda _prompt: "2")
    monkeypatch.setattr(
        interactive,
        "ask_agent_command",
        lambda *_args, **_kwargs: pytest.fail("不应再次询问 AI 命令"),
    )
    monkeypatch.setattr("subprocess.Popen", FakePopen)

    interactive._exam_wrong(FakeService(), "基金", 1)

    assert "--fix-wrong" in captured["cmd"]
    assert "--agent-command" not in captured["cmd"]
