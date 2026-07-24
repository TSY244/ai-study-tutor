"""Tests for the ai-study-tutor (OpenAI-compatible API, src/ architecture)."""

from __future__ import annotations

import json
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from src.material_parse.local_parse import clean_subtitle_text
from src.material_parse.url_parse import is_bilibili, is_youtube
from src.storage import Storage, slugify
from src.chunking import clean_text, chapter_slice
from src.memory import update_weaknesses, weakness_id
from src.config_loader import Config, ConfigError
from src.models import LessonInfo, Quiz, QuizQuestion, QuestionType


# ---------------------------------------------------------------------------
# Storage
# ---------------------------------------------------------------------------

def test_slugify_keeps_chinese_and_normalizes_spaces():
    assert slugify("投资基金入门") == "投资基金入门"
    assert slugify("Python Basics 101!") == "python-basics-101"
    assert slugify("  高数  ") == "高数"


def test_storage_creates_directories(tmp_path: Path):
    storage = Storage(tmp_path / "course")
    assert storage.course_root.exists()
    assert storage.list_subjects() == []


def test_storage_list_subjects_and_lessons(tmp_path: Path):
    storage = Storage(tmp_path / "course")
    storage.ensure_lesson_dir("高数", 1)
    storage.save_json(storage.info_path("高数", 1), {"subject": "高数", "lesson": 1, "name": "极限基础"})

    assert "高数" in [slugify("高数")]
    subjects = storage.list_subjects()
    assert len(subjects) == 1

    lessons = storage.list_lessons("高数")
    assert lessons == [1]
    assert storage.lesson_exists("高数", 1)


# ---------------------------------------------------------------------------
# Subtitle cleaning
# ---------------------------------------------------------------------------

def test_clean_subtitle_text_removes_srt_and_vtt_timestamps():
    subtitle = """WEBVTT

1
00:00:01,000 --> 00:00:03,000
欢迎学习基金

00:00:03.500 --> 00:00:05.000
今天讲净值
"""
    cleaned = clean_subtitle_text(subtitle)
    assert "00:00" not in cleaned
    assert "欢迎学习基金" in cleaned
    assert "今天讲净值" in cleaned


# ---------------------------------------------------------------------------
# URL detection
# ---------------------------------------------------------------------------

def test_is_bilibili():
    assert is_bilibili("https://www.bilibili.com/video/BV1xx")
    assert not is_bilibili("https://www.youtube.com/watch?v=xxx")


def test_is_youtube():
    assert is_youtube("https://www.youtube.com/watch?v=xxx")
    assert is_youtube("https://youtu.be/xxxxx")
    assert not is_youtube("https://www.bilibili.com/video/BV1xx")


# ---------------------------------------------------------------------------
# Chunking
# ---------------------------------------------------------------------------

def test_clean_text_normalizes_whitespace():
    text = "hello   world\n\n\n\nfoo\r\nbar"
    result = clean_text(text)
    assert "hello world" in result
    assert "\n\n" in result
    assert "foo" in result
    assert "bar" in result


def test_chapter_slice_finds_by_title():
    raw = "第一章 极限\n这是极限的内容。\n第二章 导数\n这是导数的内容。"
    outline = {
        "chapters": [
            {"title": "第一章 极限"},
            {"title": "第二章 导数"},
        ]
    }
    chunk = chapter_slice(raw, outline, 1)
    assert "极限" in chunk
    assert "导数" not in chunk

    chunk2 = chapter_slice(raw, outline, 2)
    assert "导数" in chunk2


# ---------------------------------------------------------------------------
# Memory / Weakness tracking
# ---------------------------------------------------------------------------

def test_weakness_id_deterministic():
    wid1 = weakness_id("高数", 1, "极限定义")
    wid2 = weakness_id("高数", 1, "极限定义")
    assert wid1 == wid2
    assert len(wid1) == 12


def test_update_weaknesses_merges_existing_concept(tmp_path: Path):
    storage = Storage(tmp_path / "course")
    update_weaknesses(
        storage,
        "高数",
        [
            {
                "concept": "极限定义",
                "weakness_description": "混淆数列极限和函数极限",
                "evidence": "第 2 题答错",
                "severity": 3,
            }
        ],
    )
    update_weaknesses(
        storage,
        "高数",
        [
            {
                "concept": "极限定义",
                "weakness_description": "仍不清楚 ε-δ 语言",
                "evidence": "复测简答题失分",
                "severity": 5,
            }
        ],
    )

    data = json.loads(storage.weaknesses_path().read_text(encoding="utf-8"))
    assert len(data) == 1
    assert data[0]["review_count"] == 2
    assert data[0]["severity"] == 5
    assert "复测简答题失分" in data[0]["evidence"]


# ---------------------------------------------------------------------------
# Config loader
# ---------------------------------------------------------------------------

def test_config_loads_from_json(tmp_path: Path):
    config_path = tmp_path / "test_config.json"
    config_path.write_text(json.dumps({
        "ai_config": {
            "default_provider": "openai_compat",
            "providers": {
                "openai_compat": {
                    "api_key": "test-key",
                    "base_url": "https://api.example.com/v1",
                    "model": "gpt-4.1-mini",
                }
            },
            "max_token": 4096,
        },
        "default_course_path": "./course",
        "score_rule": {"choice": 2, "judge": 1, "short_answer": 5},
        "download_sub_only": True,
    }), encoding="utf-8")

    cfg = Config(config_path)
    assert cfg.default_provider == "openai_compat"
    assert cfg.max_token == 4096
    assert cfg.score_rule == {"choice": 2, "judge": 1, "short_answer": 5}


def test_config_raises_on_missing_file():
    with pytest.raises(ConfigError):
        Config("nonexistent_config.json")


def test_config_env_override(tmp_path: Path, monkeypatch):
    config_path = tmp_path / "test_config.json"
    config_path.write_text(json.dumps({
        "ai_config": {
            "default_provider": "openai_compat",
            "providers": {
                "openai_compat": {
                    "api_key": "config-key",
                    "base_url": "https://api.example.com/v1",
                    "model": "gpt-4.1-mini",
                }
            },
            "max_token": 4096,
        },
        "default_course_path": "./course",
        "score_rule": {"choice": 2, "judge": 1, "short_answer": 5},
        "download_sub_only": True,
    }), encoding="utf-8")

    monkeypatch.setenv("OPENAI_API_KEY", "env-key")
    monkeypatch.setenv("OPENAI_MODEL", "deepseek-v4-pro")

    cfg = Config(config_path)
    provider_cfg = cfg.provider_config("openai_compat")
    assert provider_cfg["api_key"] == "env-key"
    assert provider_cfg["model"] == "deepseek-v4-pro"


def test_config_model_override(tmp_path: Path):
    """CLI --model flag has highest priority."""
    config_path = tmp_path / "test_config.json"
    config_path.write_text(json.dumps({
        "ai_config": {
            "default_provider": "openai_compat",
            "providers": {
                "openai_compat": {
                    "api_key": "config-key",
                    "base_url": "https://api.example.com/v1",
                    "model": "gpt-4.1-mini",
                }
            },
            "max_token": 4096,
        },
        "default_course_path": "./course",
        "score_rule": {"choice": 2, "judge": 1, "short_answer": 5},
        "download_sub_only": True,
    }), encoding="utf-8")

    cfg = Config(config_path)
    provider_cfg = cfg.provider_config("openai_compat", model_override="claude-opus-4-8")
    assert provider_cfg["model"] == "claude-opus-4-8"


def test_config_agent_command_defaults_to_codex_command(tmp_path: Path):
    from src.config_loader import CODEX_AGENT_COMMAND, DEFAULT_AGENT_COMMAND

    config_path = tmp_path / "test_config.json"
    config_path.write_text(json.dumps({
        "ai_config": {
            "default_provider": "openai_compat",
            "providers": {
                "openai_compat": {
                    "api_key": "config-key",
                    "base_url": "https://api.example.com/v1",
                    "model": "gpt-4.1-mini",
                }
            },
            "max_token": 4096,
        },
        "default_course_path": "./course",
    }), encoding="utf-8")

    cfg = Config(config_path)

    assert DEFAULT_AGENT_COMMAND == CODEX_AGENT_COMMAND
    assert cfg.agent_command == CODEX_AGENT_COMMAND


def test_config_agent_command_resolves_claude_preset(tmp_path: Path):
    from src.config_loader import CLAUDE_AGENT_COMMAND

    config_path = tmp_path / "test_config.json"
    config_path.write_text(json.dumps({
        "agent_command": "claude",
        "ai_config": {
            "default_provider": "openai_compat",
            "providers": {
                "openai_compat": {
                    "api_key": "config-key",
                    "base_url": "https://api.example.com/v1",
                    "model": "gpt-4.1-mini",
                }
            },
            "max_token": 4096,
        },
        "default_course_path": "./course",
    }), encoding="utf-8")

    cfg = Config(config_path)

    assert cfg.agent_command == CLAUDE_AGENT_COMMAND


def test_config_agent_command_reads_plain_string(tmp_path: Path):
    config_path = tmp_path / "test_config.json"
    config_path.write_text(json.dumps({
        "agent_command": "claude code --deepseek-pro",
        "ai_config": {
            "default_provider": "openai_compat",
            "providers": {
                "openai_compat": {
                    "api_key": "config-key",
                    "base_url": "https://api.example.com/v1",
                    "model": "gpt-4.1-mini",
                }
            },
            "max_token": 4096,
        },
        "default_course_path": "./course",
    }), encoding="utf-8")

    cfg = Config(config_path)

    assert cfg.agent_command == "claude code --deepseek-pro"


# ---------------------------------------------------------------------------
# AI adapter (OpenAI-compatible)
# ---------------------------------------------------------------------------

def test_ai_adapter_creation():
    from src.ai_adapter.claude_api import ClaudeAPI

    adapter = ClaudeAPI({
        "api_key": "test-key",
        "base_url": "https://api.example.com/v1",
        "model": "deepseek-v4-pro",
        "max_token": 4096,
    })
    assert adapter.model == "deepseek-v4-pro"
    assert adapter.base_url == "https://api.example.com/v1"


def test_ai_adapter_missing_key_error():
    from src.ai_adapter.claude_api import ClaudeAPI, AIAPIError

    adapter = ClaudeAPI({
        "api_key": "替换为你的API Key",
        "base_url": "https://api.example.com/v1",
        "model": "gpt-4.1-mini",
    })
    with pytest.raises(AIAPIError, match="请先设置 API Key"):
        adapter.chat("hello")


@patch("src.ai_adapter.claude_api.OpenAI")
def test_ai_adapter_openai_call(mock_openai_class):
    from src.ai_adapter.claude_api import ClaudeAPI

    # We also need to ensure the module-level import works with our mock
    import src.ai_adapter.claude_api as claude_mod
    claude_mod.OpenAI = mock_openai_class

    mock_client = MagicMock()
    mock_choice = MagicMock()
    mock_choice.message.content = "Hello, world!"
    mock_client.chat.completions.create.return_value.choices = [mock_choice]
    mock_openai_class.return_value = mock_client

    adapter = ClaudeAPI({
        "api_key": "test-key",
        "base_url": "https://api.example.com/v1",
        "model": "deepseek-v4-pro",
    })
    result = adapter.chat("Hi")
    assert result == "Hello, world!"

    # Verify the client was created with correct params
    mock_openai_class.assert_called_once_with(
        api_key="test-key",
        base_url="https://api.example.com/v1",
    )
    # Verify the completions.create call
    call_kwargs = mock_client.chat.completions.create.call_args.kwargs
    assert call_kwargs["model"] == "deepseek-v4-pro"


# ---------------------------------------------------------------------------
# Question scoring logic
# ---------------------------------------------------------------------------

def test_objective_question_scoring():
    from src.exam_logic import is_objective_correct

    q = QuizQuestion(
        id="q1",
        type=QuestionType.single_choice,
        question="What is 1+1?",
        options={"A": "1", "B": "2", "C": "3", "D": "4"},
        correct_answer="B",
        explanation="1+1=2",
        tested_concepts=["math"],
        score=5,
    )
    assert is_objective_correct(q, "B")
    assert not is_objective_correct(q, "A")

    q2 = QuizQuestion(
        id="q2",
        type=QuestionType.true_false,
        question="1+1=2?",
        correct_answer="Y",
        explanation="Yes",
        tested_concepts=["math"],
        score=1,
    )
    assert is_objective_correct(q2, "Y")
    assert is_objective_correct(q2, "YES")
    assert is_objective_correct(q2, "对")
    assert not is_objective_correct(q2, "N")

    q3 = QuizQuestion(
        id="q3",
        type=QuestionType.short_answer,
        question="Explain why",
        correct_answer="Because...",
        explanation="...",
        tested_concepts=["reasoning"],
        score=5,
    )
    assert not is_objective_correct(q3, "Because...")


# ---------------------------------------------------------------------------
# Quiz I/O
# ---------------------------------------------------------------------------

def test_quiz_to_markdown():
    from src.exam_logic import quiz_to_markdown
    from src.models import Quiz

    quiz = Quiz(
        id="quiz_01",
        course_id="高数",
        chapter_id=1,
        title="测试",
        questions=[
            QuizQuestion(
                id="q1",
                type=QuestionType.single_choice,
                question="1+1=?",
                options={"A": "1", "B": "2", "C": "3"},
                correct_answer="B",
                explanation="基础",
                tested_concepts=["加法"],
                score=5,
            )
        ],
    )
    md = quiz_to_markdown(quiz)
    assert "测试" in md
    assert "1+1=?" in md
    assert "加法" in md


def test_quiz_to_markdown_teacher():
    from src.exam_logic import quiz_to_markdown
    from src.models import Quiz, QuizQuestion

    quiz = Quiz(
        id="quiz_01",
        course_id="高数",
        chapter_id=1,
        title="测试",
        questions=[
            QuizQuestion(
                id="q1",
                type=QuestionType.single_choice,
                question="1+1=?",
                options={"A": "1", "B": "2"},
                correct_answer="B",
                explanation="基础加法",
                tested_concepts=["加法"],
                score=5,
            )
        ],
    )
    md_teacher = quiz_to_markdown(quiz, teacher=True)
    assert "B" in md_teacher
    assert "基础加法" in md_teacher


# ---------------------------------------------------------------------------
# Answer parsing
# ---------------------------------------------------------------------------

def test_parse_answers_markdown(tmp_path: Path):
    from src.exam_logic import _parse_answers_markdown

    answers_path = tmp_path / "answers.md"
    answers_path.write_text("q1: A\nq2: hello world\nq3:\nmulti\nline\n", encoding="utf-8")

    answers = _parse_answers_markdown(answers_path)
    assert len(answers) == 3
    assert answers[0].user_answer == "A"
    assert answers[1].user_answer == "hello world"
    assert "multi" in answers[2].user_answer


# ---------------------------------------------------------------------------
# Local file parsing (text only - no media)
# ---------------------------------------------------------------------------

def test_parse_local_txt_file(tmp_path: Path):
    from src.material_parse.local_parse import parse_local_file

    f = tmp_path / "test.txt"
    f.write_text("极限是微积分的基础概念。\n\n它描述了函数在某一点附近的变化趋势。", encoding="utf-8")

    text = parse_local_file(f)
    assert "极限" in text
    assert "微积分" in text


def test_parse_local_file_not_found():
    from src.material_parse.local_parse import parse_local_file, ParseError

    with pytest.raises(ParseError, match="文件不存在"):
        parse_local_file("/nonexistent/file.pdf")


def test_parse_unsupported_extension(tmp_path: Path):
    from src.material_parse.local_parse import parse_local_file, ParseError

    f = tmp_path / "test.xyz"
    f.write_text("hello", encoding="utf-8")

    with pytest.raises(ParseError, match="暂不支持"):
        parse_local_file(f)


# ---------------------------------------------------------------------------
# Lesson info model
# ---------------------------------------------------------------------------

def test_lesson_info_model():
    info = LessonInfo(subject="高数", lesson=1, name="极限基础", raw_text="极限是...")
    assert info.subject == "高数"
    assert info.lesson == 1

    data = info.model_dump(mode="json")
    assert data["subject"] == "高数"
    assert "created_at" in data


# ---------------------------------------------------------------------------
# Course build (mock AI)
# ---------------------------------------------------------------------------

def test_build_doc_mocked(tmp_path: Path):
    from src.course_build import build_doc

    mock_ai = MagicMock()
    mock_ai.chat.side_effect = [
        "# 教师教纲\n\n## 教学目标\n...",
        "# 学生笔记\n\n## 详细讲解\n...",
    ]

    storage = Storage(tmp_path / "course")
    teacher, student = build_doc(
        "极限是微积分的基础。", "高数", 1, "极限基础", mock_ai, storage
    )

    assert "教师教纲" in teacher
    assert "学生笔记" in student
    assert storage.lesson_exists("高数", 1)
    assert storage.teacher_notes_path("高数", 1).exists()
    assert storage.student_notes_path("高数", 1).exists()
    assert storage.info_path("高数", 1).exists()


# ---------------------------------------------------------------------------
# Summary check (mock AI)
# ---------------------------------------------------------------------------

def test_evaluate_summary_mocked(tmp_path: Path):
    from src.summary_check import evaluate_summary

    mock_ai = MagicMock()
    mock_ai.chat_json.return_value = json.dumps({
        "accuracy_score": 85,
        "completeness_score": 70,
        "overall_assessment": "总体理解较好",
        "key_misunderstandings": ["极限定义不准确"],
        "missing_concepts": ["ε-δ语言"],
        "strengths": ["掌握了基本思想"],
        "suggestions": ["建议多做习题"],
    }, ensure_ascii=False)

    storage = Storage(tmp_path / "course")
    storage.save_text(
        storage.raw_text_path("高数", 1),
        "极限是微积分的基础概念..."
    )

    result = evaluate_summary("高数", 1, "极限就是趋近", mock_ai, storage)
    assert result["accuracy_score"] == 85
    assert len(result["missing_concepts"]) == 1


# ---------------------------------------------------------------------------
# Wrong questions listing
# ---------------------------------------------------------------------------

def test_list_wrong_questions(tmp_path: Path):
    from src.exam_logic import list_wrong_questions

    storage = Storage(tmp_path / "course")
    wdir = storage.wrong_questions_dir("高数", 1)
    storage.save_json(wdir / "20240601_120000_wrong.json", [
        {"question_id": "q1", "question": "1+1=?", "user_answer": "A", "correct_answer": "B"},
    ])

    wrong = list_wrong_questions("高数", 1, storage)
    assert len(wrong) == 1
    assert wrong[0]["question"] == "1+1=?"


# ---------------------------------------------------------------------------
# Score recording
# ---------------------------------------------------------------------------

def test_record_and_get_scores(tmp_path: Path):
    from src.exam_logic import record_score, get_scores

    storage = Storage(tmp_path / "course")
    record_score("高数", 1, 85.0, 100.0, storage)

    scores = get_scores("高数", 1, storage)
    assert len(scores) == 1
    assert scores[0]["score"] == 85.0


def test_grade_answers_reads_saved_json_attempt_and_updates_weaknesses(tmp_path: Path):
    from src.exam_logic import grade_answers, list_pending_grading_attempts, save_quiz_files

    storage = Storage(tmp_path / "course")
    quiz = Quiz(
        id="quiz_chapter_01",
        course_id="高数",
        chapter_id=1,
        title="极限测试",
        questions=[
            QuizQuestion(
                id="q1",
                type=QuestionType.short_answer,
                question="什么是极限？",
                correct_answer="函数值随自变量趋近某点而趋近的值。",
                explanation="极限描述趋近过程中的稳定趋势。",
                tested_concepts=["极限定义"],
                score=5,
            )
        ],
    )
    save_quiz_files(storage, "高数", quiz)
    answer_path = storage.exam_dir("高数", 1) / "20260619_120000_answers.json"
    storage.save_json(answer_path, {
        "quiz_id": quiz.id,
        "course_id": "高数",
        "chapter_id": 1,
        "answers": [
            {"question_id": "q1", "user_answer": "就是越来越近", "is_correct": False}
        ],
    })

    class MockAI:
        def chat_json(self, prompt: str, system: str = "") -> str:
            return json.dumps({
                "total_score": 2,
                "question_feedback": [
                    {
                        "question_id": "q1",
                        "score": 2,
                        "is_correct": False,
                        "feedback": "答案提到了趋近，但缺少函数值和稳定趋势。",
                        "correct_understanding": "极限强调自变量趋近时函数值的趋近目标。",
                        "weak_concepts": ["极限定义"],
                    }
                ],
                "weak_concepts": [
                    {
                        "concept": "极限定义",
                        "weakness_description": "只记住了趋近，没说明函数值趋近的对象。",
                        "severity": 4,
                        "evidence": "q1 主观题回答不完整",
                    }
                ],
                "review_plan": ["复习极限定义并写出完整表述。"],
                "retest_recommended": True,
            }, ensure_ascii=False)

    attempt = grade_answers("高数", quiz.id, answer_path, MockAI(), storage)

    assert attempt.total_score == 2
    feedback_path = storage.exam_dir("高数", 1) / "20260619_120000_feedback.json"
    assert feedback_path.exists()
    assert (storage.exam_dir("高数", 1) / "20260619_120000_feedback.md").exists()
    answer_record = storage.read_json(answer_path)
    assert answer_record["answers"][0]["ai_is_correct"] is False
    assert answer_record["answers"][0]["ai_score"] == 2
    assert answer_record["answers"][0]["weak_concepts"] == ["极限定义"]
    assert answer_record["ai_grading"]["status"] == "completed"
    assert answer_record["ai_grading"]["total_score"] == 2
    assert answer_record["ai_grading"]["weak_concepts"][0]["concept"] == "极限定义"
    weaknesses = storage.read_json(storage.weaknesses_path())
    assert weaknesses[0]["concept"] == "极限定义"
    assert list_pending_grading_attempts("高数", 1, storage) == []


def test_run_cli_quiz_marks_attempt_as_pending_ai_grading(tmp_path: Path, monkeypatch):
    from src.exam_logic import list_pending_grading_attempts, run_cli_quiz

    storage = Storage(tmp_path / "course")
    quiz = Quiz(
        id="quiz_chapter_01",
        course_id="基金",
        chapter_id=1,
        title="基金测试",
        questions=[
            QuizQuestion(
                id="q1",
                type=QuestionType.single_choice,
                question="基金托管人通常是谁？",
                options={"A": "基金管理人", "B": "商业银行"},
                correct_answer="B",
                explanation="基金托管人通常由商业银行担任。",
                tested_concepts=["基金托管人"],
                score=5,
            )
        ],
    )
    monkeypatch.setattr("builtins.input", lambda _prompt: "B")

    attempt_path, _score, _wrong = run_cli_quiz(storage, "基金", quiz)

    attempt = storage.read_json(attempt_path)
    assert attempt["ai_grading"]["status"] == "pending"
    pending = list_pending_grading_attempts("基金", 1, storage)
    assert len(pending) == 1
    assert pending[0]["status"] == "pending"
    assert pending[0]["answers_path"] == str(attempt_path)


def test_mark_grading_failed_records_retry_metadata(tmp_path: Path):
    from src.exam_logic import list_pending_grading_attempts, mark_grading_failed

    storage = Storage(tmp_path / "course")
    answer_path = storage.exam_dir("基金", 1) / "20260619_121500_answers.json"
    storage.save_json(answer_path, {
        "quiz_id": "quiz_chapter_01",
        "course_id": "基金",
        "chapter_id": 1,
        "answers": [],
        "ai_grading": {"status": "pending"},
    })

    mark_grading_failed(storage, answer_path, RuntimeError("API 失败"))

    attempt = storage.read_json(answer_path)
    assert attempt["ai_grading"]["status"] == "failed"
    assert attempt["ai_grading"]["retryable"] is True
    assert "API 失败" in attempt["ai_grading"]["error"]
    pending = list_pending_grading_attempts("基金", 1, storage)
    assert len(pending) == 1
    assert pending[0]["status"] == "failed"
    assert "API 失败" in pending[0]["error"]


def test_list_pending_grading_attempts_can_include_completed_for_regrading(tmp_path: Path):
    from src.exam_logic import list_pending_grading_attempts

    storage = Storage(tmp_path / "course")
    answer_path = storage.exam_dir("基金", 1) / "20260619_121500_answers.json"
    feedback_path = storage.exam_dir("基金", 1) / "20260619_121500_feedback.json"
    storage.save_json(answer_path, {
        "quiz_id": "quiz_chapter_01",
        "course_id": "基金",
        "chapter_id": 1,
        "answers": [],
        "ai_grading": {"status": "completed"},
    })
    storage.save_json(feedback_path, {"feedback": "旧点评"})

    assert list_pending_grading_attempts("基金", 1, storage) == []

    regradeable = list_pending_grading_attempts(
        "基金", 1, storage, include_completed=True
    )
    assert len(regradeable) == 1
    assert regradeable[0]["status"] == "completed"
    assert regradeable[0]["answers_path"] == str(answer_path)


def test_cmd_process_tasks_uses_claude_cli_agent(monkeypatch, capsys):
    import main
    from src import agent_runner

    class Args:
        agent_command = "claude code --deepseek-pro"

    calls: dict[str, object] = {}
    monkeypatch.setattr(agent_runner, "list_pending_tasks", lambda: [
        {"task_id": "task_1", "task_type": "summarize", "subject": "高数", "name": "总结"}
    ])

    def fake_process_all(agent_command=None):
        calls["agent_command"] = agent_command
        return (1, 0)

    monkeypatch.setattr(agent_runner, "process_all_pending_via_claude", fake_process_all)

    def fail_direct():
        raise AssertionError("不应调用直连 API 处理任务")

    monkeypatch.setattr(agent_runner, "process_pending_tasks_now", fail_direct)

    main.cmd_process_tasks(Args())

    assert calls["agent_command"] == "claude code --deepseek-pro"
    assert "成功 1 个，失败 0 个" in capsys.readouterr().out


def test_queue_grade_attempt_task_uses_lesson_notes_and_saved_attempt(tmp_path: Path, monkeypatch):
    from src import agent_runner
    from src.agent_runner import queue_grade_attempt_task

    monkeypatch.setattr(agent_runner, "TASKS_DIR", tmp_path / ".tasks")
    monkeypatch.setattr(agent_runner, "PENDING_DIR", tmp_path / ".tasks" / "pending")
    monkeypatch.setattr(agent_runner, "COMPLETED_DIR", tmp_path / ".tasks" / "completed")

    storage = Storage(tmp_path / "course")
    storage.save_text(storage.teacher_notes_path("基金", 3), "# 教师教纲\n托管人负责资金保管。")
    storage.save_text(storage.student_notes_path("基金", 3), "# 学生笔记\n基金托管人通常是商业银行。")
    storage.save_json(storage.exam_dir("基金", 3) / "quiz_chapter_03.json", {
        "id": "quiz_chapter_03",
        "course_id": "基金",
        "chapter_id": 3,
        "title": "基金测试",
        "questions": [],
    })
    answer_path = storage.exam_dir("基金", 3) / "20260620_115723_answers.json"
    storage.save_json(answer_path, {
        "quiz_id": "quiz_chapter_03",
        "course_id": "基金",
        "chapter_id": 3,
        "answers": [{"question_id": "q1", "user_answer": "商业银行"}],
    })

    task_id = queue_grade_attempt_task("基金", 3, "quiz_chapter_03", answer_path, storage)
    task_path = agent_runner.PENDING_DIR / f"{task_id}.json"
    task = json.loads(task_path.read_text(encoding="utf-8"))

    assert task["task_type"] == "grade_attempt"
    assert task["output_format"] == "json"
    assert "20260620_115723_grade_result.json" in task["output_files"]
    assert "教师教纲" in task["prompt"]
    assert "学生笔记" in task["prompt"]
    assert str(answer_path) in task["prompt"]


def test_grade_attempt_with_claude_cli_uses_agent_task(monkeypatch, tmp_path: Path):
    from src import agent_runner
    from src.exam_logic import grade_attempt_with_claude_cli

    storage = Storage(tmp_path / "course")
    answer_path = storage.exam_dir("基金", 3) / "20260620_115723_answers.json"
    storage.save_json(answer_path, {"quiz_id": "quiz_chapter_03", "answers": []})
    calls: dict[str, object] = {}

    def fake_queue(subject, lesson, quiz_id, answers_path, storage, model=""):
        calls["queue"] = (subject, lesson, quiz_id, str(answers_path), model)
        return "task_grade_123"

    def fake_process(task_id, timeout=300):
        calls["process"] = (task_id, timeout)
        return True

    monkeypatch.setattr(agent_runner, "queue_grade_attempt_task", fake_queue)
    monkeypatch.setattr(agent_runner, "process_task_via_claude_cli", fake_process)

    assert grade_attempt_with_claude_cli(
        "基金", 3, "quiz_chapter_03", answer_path, storage, model="deepseek-v4-pro"
    ) is True
    assert calls["queue"] == ("基金", 3, "quiz_chapter_03", str(answer_path), "deepseek-v4-pro")
    assert calls["process"] == ("task_grade_123", 600)


def test_cmd_grade_pending_passes_agent_command(monkeypatch, tmp_path: Path):
    import main

    class FakeConfig:
        course_path = tmp_path / "course"
        agent_command = "claude -p"

    class Args:
        subject = "基金"
        lesson = "3"
        agent_command = "claude code --deepseek-pro"

    calls: dict[str, object] = {}

    monkeypatch.setattr("src.config_loader.get_config", lambda: FakeConfig())
    monkeypatch.setattr(main, "list_pending_grading_attempts", lambda *_args: [
        {
            "quiz_id": "quiz_chapter_03",
            "answers_path": str(tmp_path / "answers.json"),
            "status": "pending",
        }
    ])
    monkeypatch.setattr(main, "mark_grading_failed", lambda *_args: None)

    def fake_grade(subject, lesson, quiz_id, answers_path, storage, agent_command=""):
        calls["grade"] = (subject, lesson, quiz_id, answers_path, agent_command)
        return False

    monkeypatch.setattr(main, "grade_attempt_with_claude_cli", fake_grade)

    main.cmd_grade_pending(Args())

    assert calls["grade"] == (
        "基金",
        3,
        "quiz_chapter_03",
        str(tmp_path / "answers.json"),
        "claude code --deepseek-pro",
    )


def test_cmd_exam_generates_wrong_analysis_after_quiz(
    monkeypatch, tmp_path: Path, capsys
):
    import main

    storage = Storage(tmp_path / "course")
    storage.save_json(
        storage.info_path("基金", 1),
        {"subject": "基金", "lesson": 1, "name": "金字塔定投法"},
    )
    attempt_path = storage.exam_dir("基金", 1) / "20260623_120000_answers.json"
    storage.save_json(
        attempt_path,
        {
            "quiz_id": "quiz_chapter_01",
            "course_id": "基金",
            "chapter_id": 1,
            "answers": [],
        },
    )
    quiz = Quiz(
        id="quiz_chapter_01",
        course_id="基金",
        chapter_id=1,
        title="基金测试",
        questions=[
            QuizQuestion(
                id="q1",
                type=QuestionType.single_choice,
                question="金字塔定投的加仓方向是什么？",
                options={"A": "越涨越买", "B": "越跌越买"},
                correct_answer="B",
                explanation="金字塔定投通常在价格下跌时逐步增加投入。",
            )
        ],
    )
    wrong_answers = [
        {
            "question_id": "q1",
            "question": "金字塔定投的加仓方向是什么？",
            "user_answer": "A",
            "correct_answer": "B",
        }
    ]

    class FakeConfig:
        course_path = tmp_path / "course"
        agent_command = "claude -p"

    class Args:
        subject = "基金"
        lesson = "1"
        quiz_id = None
        agent_command = None

    calls: dict[str, object] = {}

    monkeypatch.setattr("src.config_loader.get_config", lambda: FakeConfig())
    monkeypatch.setattr(
        main,
        "list_historical_quizzes",
        lambda *_args: [{"id": "quiz_chapter_01", "title": "基金测试"}],
    )
    monkeypatch.setattr(main, "load_quiz_by_id", lambda *_args: quiz)
    monkeypatch.setattr(
        main,
        "run_cli_quiz",
        lambda *_args: (attempt_path, 0.0, wrong_answers),
    )
    monkeypatch.setattr(main, "grade_attempt_with_claude_cli", lambda *_args, **_kwargs: False)
    monkeypatch.setattr(main, "mark_grading_failed", lambda *_args, **_kwargs: None)

    def fake_wrong_analysis(
        subject: str,
        lesson: int,
        wrong_list: list[dict],
        storage_arg: Storage,
        *,
        agent_command: str = "",
        attempt_path: Path | str | None = None,
    ) -> Path:
        output_path = storage_arg.wrong_questions_dir(subject, lesson) / "20260623_120000_wrong_analysis.md"
        storage_arg.save_text(output_path, "# 错题解析")
        calls["wrong_analysis"] = (
            subject,
            lesson,
            wrong_list,
            agent_command,
            Path(attempt_path) if attempt_path else None,
        )
        return output_path

    monkeypatch.setattr(main, "analyze_wrong_answers_with_claude_cli", fake_wrong_analysis)

    main.cmd_exam(Args())

    assert calls["wrong_analysis"] == (
        "基金",
        1,
        wrong_answers,
        "claude -p",
        attempt_path,
    )
    out = capsys.readouterr().out
    assert "错题解析" in out
    assert "20260623_120000_wrong_analysis.md" in out


def test_build_agent_command_splits_string_and_appends_prompt():
    from src.agent_runner import build_agent_command

    command = build_agent_command(
        "请处理任务",
        agent_command='claude code --profile "deep seek" --deepseek-pro',
    )

    assert command == [
        "claude",
        "code",
        "--profile",
        "deep seek",
        "--deepseek-pro",
        "请处理任务",
    ]


def test_build_agent_command_resolves_agent_preset():
    from src.agent_runner import build_agent_command
    from src.config_loader import CLAUDE_AGENT_COMMAND

    command = build_agent_command("请处理任务", agent_command="claude")

    assert command == [*CLAUDE_AGENT_COMMAND.split(), "请处理任务"]


def test_process_task_uses_task_agent_command(monkeypatch, tmp_path: Path):
    from types import SimpleNamespace

    from src import agent_runner

    monkeypatch.setattr(agent_runner, "TASKS_DIR", tmp_path / ".tasks")
    monkeypatch.setattr(agent_runner, "PENDING_DIR", tmp_path / ".tasks" / "pending")
    monkeypatch.setattr(agent_runner, "COMPLETED_DIR", tmp_path / ".tasks" / "completed")
    agent_runner.PENDING_DIR.mkdir(parents=True)

    output_dir = tmp_path / "course" / "基金" / "lesson_01" / "exam"
    output_dir.mkdir(parents=True)
    task_id = "task_20260622_quiz_gen_abc123"
    task_path = agent_runner.PENDING_DIR / f"{task_id}.json"
    task_path.write_text(json.dumps({
        "task_id": task_id,
        "task_type": "quiz_gen",
        "subject": "基金",
        "lesson": 1,
        "prompt": "生成试卷",
        "output_dir": str(output_dir),
        "output_files": ["quiz_result.json"],
        "output_format": "json",
        "command": "claude code --deepseek-pro",
        "status": "pending",
        "created_at": "2026-06-22T00:00:00+00:00",
        "completed_at": None,
        "error": None,
        "extra": {},
    }, ensure_ascii=False), encoding="utf-8")

    captured: dict[str, list[str]] = {}

    def fake_run(cmd, **_kwargs):
        captured["cmd"] = cmd
        result_path = output_dir / "quiz_chapter_01_abc123.json"
        result_path.write_text(json.dumps({
            "id": "quiz_chapter_01_abc123",
            "course_id": "基金",
            "chapter_id": 1,
            "title": "基金·第1课 章节测试",
            "questions": [],
        }, ensure_ascii=False), encoding="utf-8")
        return SimpleNamespace(returncode=0, stdout="", stderr="")

    monkeypatch.setattr("subprocess.run", fake_run)

    assert agent_runner.process_task_via_claude_cli(task_id) is True
    assert captured["cmd"][:3] == ["claude", "code", "--deepseek-pro"]
    assert captured["cmd"][-1].startswith("请处理任务文件")


# ---------------------------------------------------------------------------
# Extraction runner (local claude CLI)
# ---------------------------------------------------------------------------

def test_build_extraction_prompt():
    from src.extraction_runner import build_extraction_prompt

    prompt = build_extraction_prompt(
        "https://youtu.be/test 测试链接",
        "数学", 1, "极限基础",
    )
    assert "数学" in prompt
    assert "极限基础" in prompt
    assert "teacher_notes" in prompt
    assert "student_notes" in prompt
    assert "EXTRACTION_COMPLETE" in prompt


def test_run_extraction_requires_model():
    from src.extraction_runner import run_extraction, ExtractionError

    with pytest.raises(ExtractionError, match="必须指定 --model"):
        run_extraction(
            prompt="echo hello",
            output_dir=Path("/tmp/test"),
            model="",
            api_key="test",
        )


def test_create_subject_persists_without_lessons(tmp_path: Path):
    storage = Storage(tmp_path / "course")
    storage.create_subject("高数")
    assert "高数" in storage.list_subjects()
    assert storage.list_lessons("高数") == []

    storage.create_subject("英语")
    assert len(storage.list_subjects()) == 2


def test_delete_lesson_and_subject(tmp_path: Path):
    storage = Storage(tmp_path / "course")
    storage.ensure_lesson_dir("高数", 1)
    storage.save_text(storage.raw_text_path("高数", 1), "测试内容")
    storage.save_json(storage.info_path("高数", 1), {"subject": "高数", "lesson": 1, "name": "测试"})

    assert storage.lesson_exists("高数", 1)
    assert storage.delete_lesson("高数", 1)
    assert not storage.lesson_exists("高数", 1)
    # Subject dir should be cleaned up since it's now empty
    assert not storage.subject_dir("高数").exists()


def test_delete_subject_recursive(tmp_path: Path):
    storage = Storage(tmp_path / "course")
    storage.ensure_lesson_dir("物理", 1)
    storage.ensure_lesson_dir("物理", 2)
    storage.save_json(storage.info_path("物理", 1), {"name": "力学"})
    storage.save_json(storage.info_path("物理", 2), {"name": "电磁学"})

    assert len(storage.list_lessons("物理")) == 2
    assert storage.delete_subject("物理")
    assert not storage.subject_dir("物理").exists()
    assert storage.list_lessons("物理") == []


def test_verify_output(tmp_path: Path):
    from src.extraction_runner import verify_output

    d = tmp_path / "out"
    d.mkdir()
    (d / "teacher_notes").mkdir()
    (d / "teacher_notes" / "content.md").write_text("# test")
    (d / "student_notes").mkdir()
    (d / "student_notes" / "content.md").write_text("# test")
    (d / "info.json").write_text("{}")

    status = verify_output(d)
    assert status["teacher_notes"] is True
    assert status["student_notes"] is True
    assert status["info_json"] is True
