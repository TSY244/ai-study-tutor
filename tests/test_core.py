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
from src.models import LessonInfo, QuizQuestion, QuestionType


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
