#!/usr/bin/env python3
"""AI 多模态课程提取工具（OpenAI 兼容 API · 非交互）

支持 B站/YouTube/本地音视频/文档素材导入，
AI 一键生成教师教纲(MD)、学生学习笔记(MD)；
附带 AI 出题、终端答题、自动归集错题、成绩统计、总结测评全链路能力。

用法：
  python main.py --init
  python main.py --load-url <URL> --subject <科目> --lesson <编号> --name <名称> [--model deepseek-v4-pro]
  python main.py --load-video <文件> --load-doc <文件> --subject <科目> --lesson <编号> --name <名称>
  python main.py --source-text <文本> --subject <科目> --lesson <编号> --name <名称>
  python main.py --subject-list
  python main.py --exam-new --subject <科目> --lesson <编号> [--num 10]
  python main.py --exam --subject <科目> --lesson <编号>
  ...
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent
sys.path.insert(0, str(PROJECT_ROOT))

from dotenv import load_dotenv

load_dotenv()

from src.config_loader import Config, ConfigError, get_config
from src.storage import Storage, StorageError
from src.exam_logic import (
    assemble_quiz,
    export_merged_md,
    get_latest_quiz,
    get_scores,
    list_all_questions,
    list_historical_quizzes,
    list_wrong_questions,
    load_quiz_by_id,
    record_score,
    run_cli_quiz,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _ensure_lesson_exists(storage, subject, lesson):
    if not storage.lesson_exists(subject, lesson):
        print(f"错误：未找到课程 {subject} 第{lesson}课。请先导入素材。")
        sys.exit(1)


# ---------------------------------------------------------------------------
# Command handlers
# ---------------------------------------------------------------------------

def cmd_init(args):
    """Initialize project directories."""
    cfg = get_config()
    storage = Storage(cfg.course_path)
    print(f"✅ 项目目录已就绪：{storage.course_root.resolve()}")
    print(f"   配置来源：{cfg.config_path.resolve()}")
    # Create default config if not exists
    config_path = Path("config.json")
    if not config_path.exists():
        import json
        default_cfg = {
            "ai_config": {
                "default_provider": "openai_compat",
                "providers": {
                    "openai_compat": {
                        "api_key": "替换为你的API Key",
                        "base_url": "https://api.openai.com/v1",
                        "model": "gpt-4.1-mini",
                    }
                },
                "max_token": 4096,
            },
            "default_course_path": "./course",
            "score_rule": {"choice": 2, "judge": 1, "short_answer": 5},
            "download_sub_only": True,
        }
        config_path.write_text(
            json.dumps(default_cfg, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        print("   已创建 config.json，请先配置 API Key 和 base_url。")
        print()
        print("   配置示例：")
        print('     "api_key": "your-key"')
        print('     "base_url": "https://your-api-gateway/v1"')
        print('     "model": "deepseek-v4-pro"')
    print()
    print("   下一步：python main.py --load-url <链接> --subject <科目> --lesson 1 --name <名称>")


def _run_extraction(source_desc: str, subject: str, lesson: int, name: str, args):
    """Queue an extraction task for Claude Code Agent."""
    from src.agent_runner import queue_extraction_task
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    lesson_dir = storage.ensure_lesson_dir(subject, lesson)
    (lesson_dir / "teacher_notes").mkdir(parents=True, exist_ok=True)
    (lesson_dir / "student_notes").mkdir(parents=True, exist_ok=True)

    model = getattr(args, 'model', None) or cfg.provider_config().get("model", "")
    if not model:
        print("错误：必须通过 --model 指定模型，或在 config.json 中配置 model 字段。")
        sys.exit(1)

    source_text = ""
    if getattr(args, 'source_text', None):
        source_text = args.source_text
        raw_path = storage.raw_text_path(subject, lesson)
        storage.save_text(raw_path, source_text)

    task_id = queue_extraction_task(
        source_desc=source_desc,
        subject=subject,
        lesson=lesson,
        name=name,
        output_dir=lesson_dir,
        model=model,
        source_text=source_text,
    )

    print(f"\n✅ 提取任务已排队：{task_id}")
    print(f"   科目：{subject} / 第{lesson}课：{name}")
    print(f"   输出目录：{lesson_dir}")
    print()

    from src.agent_runner import process_all_pending_via_claude
    succeeded, failed = process_all_pending_via_claude(timeout=900)

    if succeeded > 0:
        print(f"\n   查看结果：python main.py --learn --subject {subject} --lesson {lesson}")
    elif failed > 0:
        print()
        print("   ╔══════════════════════════════════════════════════════╗")
        print("   ║  📋 自动处理未完成，请在 Claude Code 中说：         ║")
        print("   ║     「处理所有待处理的学习任务」                    ║")
        print(f"   ║  处理完成后运行：                                   ║")
        print(f"   ║     python main.py --learn --subject {subject} --lesson {lesson}  ║")
        print("   ╚══════════════════════════════════════════════════════╝")


def cmd_load_url(args):
    """Import from B站/YouTube URL via local claude CLI."""
    subject = args.subject
    lesson = int(args.lesson)
    name = args.name or f"第{lesson}课"

    source_desc = (
        f"从视频链接提取知识：{args.load_url}\n\n"
        f"请先用 yt-dlp 获取该视频的中文字幕。如果视频有中文字幕（人工/AI），下载字幕文本；"
        f"如果没有中文字幕，尝试获取英文字幕然后翻译；"
        f"如果完全没有字幕，尝试下载视频的元数据和描述信息，结合 WebFetch 访问视频页面获取简介。"
    )
    _run_extraction(source_desc, subject, lesson, name, args)


def cmd_load_video(args):
    """Import from local video/audio + optional doc files via local claude CLI."""
    subject = args.subject
    lesson = int(args.lesson)
    name = args.name or f"第{lesson}课"

    parts: list[str] = []
    if args.load_video:
        parts.append(f"视频/音频文件：{args.load_video}（用 ffmpeg 或 yt-dlp 提取音频/字幕）")
    if args.load_doc:
        parts.append(f"文档文件：{', '.join(args.load_doc)}（直接 Read 读取内容）")
    if args.source_text:
        parts.append(f"补充文本：{args.source_text}")

    source_desc = "从本地文件提取知识：\n" + "\n".join(f"  - {p}" for p in parts)
    _run_extraction(source_desc, subject, lesson, name, args)


def cmd_load_source_text(args):
    """Import from plain text via Docker container."""
    subject = args.subject
    lesson = int(args.lesson)
    name = args.name or f"第{lesson}课"

    if not args.source_text:
        print("错误：请通过 --source-text 提供文本内容。")
        sys.exit(1)

    # Write the source text to a temp file inside the output dir so the container can read it
    _, storage, _ = _get_ai_and_storage()
    lesson_dir = storage.ensure_lesson_dir(subject, lesson)
    raw_text_path = storage.raw_text_path(subject, lesson)
    storage.save_text(raw_text_path, args.source_text)

    source_desc = (
        f"从以下文本内容提取知识（文本已保存在 /output/raw/raw_text.md，请先 Read 读取）：\n\n"
        f"科目：{subject} / 第{lesson}课：{name}\n\n"
        f"请 Read /output/raw/raw_text.md 获取完整文本，然后生成笔记。"
    )
    _run_extraction(source_desc, subject, lesson, name, args)


def cmd_subject_list(args):
    """List all subjects."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subjects = storage.list_subjects()
    if not subjects:
        print("暂无课程。请先导入素材：python main.py --load-url ...")
        return
    print("已创建科目：")
    for s in subjects:
        lessons = storage.list_lessons(s)
        print(f"  📚 {s}（{len(lessons)} 课时）")


def cmd_lessons(args):
    """List lessons for a subject."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lessons = storage.list_lessons(subject)
    if not lessons:
        print(f"科目「{subject}」暂无课时。")
        return
    print(f"科目：{subject}")
    for ln in sorted(lessons):
        info_path = storage.info_path(subject, ln)
        name = f"第{ln}课"
        if info_path.exists():
            info = storage.read_json(info_path)
            name = info.get("name", name)
        print(f"  课时 {ln}：{name}")


def cmd_learn(args):
    """View notes and optionally raw files."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)
    _ensure_lesson_exists(storage, subject, lesson)

    student_path = storage.student_notes_path(subject, lesson)
    if student_path.exists():
        print(f"\n{'='*60}")
        print(f"  学生学习笔记：{subject} 第{lesson}课")
        print(f"{'='*60}\n")
        print(storage.read_text(student_path))

    teacher_path = storage.teacher_notes_path(subject, lesson)
    if teacher_path.exists():
        print(f"\n{'='*60}")
        print(f"  教师教纲：{subject} 第{lesson}课")
        print(f"{'='*60}\n")
        print(storage.read_text(teacher_path))

    if getattr(args, 'raw', False):
        raw_path = storage.raw_text_path(subject, lesson)
        if raw_path.exists():
            print(f"\n{'='*60}")
            print(f"  原始素材")
            print(f"{'='*60}\n")
            print(storage.read_text(raw_path))


def cmd_exam_new(args):
    """AI exam generation — automatically invokes Claude Code CLI to process."""
    from src.course_service import svc
    from src.storage import Storage
    from src.config_loader import get_config
    from src.agent_runner import process_task_via_claude_cli

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)
    num = int(args.num) if args.num else None  # None → policy-driven
    _ensure_lesson_exists(storage, subject, lesson)

    model = getattr(args, 'model', None) or cfg.provider_config().get("model", "")

    # Queue the task
    task_id = svc().queue_exam_generation(subject, lesson, num, model)

    # Automatically invoke Claude Code CLI
    success = process_task_via_claude_cli(task_id)

    if success:
        print(f"\n   开始答题：python main.py --exam --subject {subject} --lesson {lesson}")
    else:
        print()
        print("   ╔══════════════════════════════════════════════════════╗")
        print("   ║  📋 自动生成未完成，请在 Claude Code 中说：         ║")
        print("   ║     「处理所有待处理的学习任务」                    ║")
        print("   ╚══════════════════════════════════════════════════════╝")


def cmd_exam(args):
    """Terminal interactive exam. Supports --quiz-id for historical quizzes."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)
    _ensure_lesson_exists(storage, subject, lesson)

    quiz: Quiz | None = None

    if args.quiz_id:
        # User specified a particular quiz
        quiz = load_quiz_by_id(subject, lesson, args.quiz_id, storage)
        if quiz is None:
            print(f"未找到试卷：{args.quiz_id}")
            print("使用 --exam-list 查看可用试卷。")
            sys.exit(1)
    else:
        # Default: latest quiz, or let user pick if multiple
        quizzes = list_historical_quizzes(subject, lesson, storage)
        if not quizzes:
            print("未找到试卷。请先生成：python main.py --exam-new --subject <科目> --lesson <编号>")
            sys.exit(1)

        if len(quizzes) == 1:
            quiz = load_quiz_by_id(subject, lesson, quizzes[0]["id"], storage)
        else:
            # Interactive pick
            print(f"\n可用试卷：{subject} 第{lesson}课\n")
            for i, qm in enumerate(quizzes, start=1):
                src_label = {"regular": "常规", "retest": "复习", "assembled": "组装"}.get(
                    qm.get("source", ""), qm.get("source", ""))
                print(f"  {i}. [{src_label}] {qm['title']}（{qm['question_count']} 题）— {qm.get('created_at', '')[:10]}")
            print()
            choice = input("选择试卷编号（默认=1）：").strip() or "1"
            try:
                idx = int(choice) - 1
                if 0 <= idx < len(quizzes):
                    quiz = load_quiz_by_id(subject, lesson, quizzes[idx]["id"], storage)
            except (ValueError, IndexError):
                pass

    if quiz is None:
        print("未能加载试卷。")
        sys.exit(1)

    from src.models import Quiz

    print(f"\n📝 开始答题：{quiz.title}")
    print(f"   共 {len(quiz.questions)} 题\n")
    attempt_path, auto_score, wrong_answers = run_cli_quiz(storage, subject, quiz)

    total_possible = sum(q.score for q in quiz.questions)
    record_score(subject, lesson, auto_score, total_possible, storage)

    if wrong_answers:
        print(f"\n错题数：{len(wrong_answers)}")
        print(f"查看错题：python main.py --wrong-list --subject {subject} --lesson {lesson}")


def cmd_exam_list(args):
    """List all historical quizzes for a subject/lesson."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)
    _ensure_lesson_exists(storage, subject, lesson)

    quizzes = list_historical_quizzes(subject, lesson, storage)
    if not quizzes:
        print(f"暂无试卷：{subject} 第{lesson}课")
        print("先生成：python main.py --exam-new --subject <科目> --lesson <编号>")
        return

    src_label = {"regular": "📝常规", "retest": "🔄复习", "assembled": "🧩组装"}

    print(f"\n历史试卷：{subject} 第{lesson}课（共 {len(quizzes)} 份）\n")
    for i, qm in enumerate(quizzes, start=1):
        label = src_label.get(qm.get("source", ""), qm.get("source", ""))
        print(f"  {i}. {label} {qm['title']}")
        print(f"     ID: {qm['id']}")
        print(f"     题目数: {qm['question_count']} | 日期: {qm.get('created_at', '?')[:10]}")
        print()

    print("使用 --quiz-id <ID> 指定试卷：")
    print(f"  python main.py --exam --subject {subject} --lesson {lesson} --quiz-id <ID>")


def cmd_exam_assemble(args):
    """Manually assemble a quiz from questions across historical quizzes."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)
    _ensure_lesson_exists(storage, subject, lesson)

    # List available questions
    all_qs = list_all_questions(subject, lesson, storage)
    if not all_qs:
        print(f"暂无题目：{subject} 第{lesson}课")
        print("先生成试卷：python main.py --exam-new --subject <科目> --lesson <编号>")
        return

    # Group by quiz
    by_quiz: dict[str, list[dict]] = {}
    for q in all_qs:
        qid = q.get("_quiz_id", "unknown")
        by_quiz.setdefault(qid, []).append(q)

    type_label = {
        "single_choice": "单选", "multiple_choice": "多选", "true_false": "判断",
        "short_answer": "简答", "case_analysis": "案例分析",
    }

    print(f"\n可用题目池：{subject} 第{lesson}课（共 {len(all_qs)} 题）\n")
    print("=" * 70)

    for quiz_id, questions in by_quiz.items():
        quiz_title = questions[0].get("_quiz_title", quiz_id)
        print(f"\n📋 [{quiz_id}] {quiz_title}（{len(questions)} 题）")
        print("-" * 60)
        for q in questions:
            tlabel = type_label.get(q.get("type", ""), q.get("type", ""))
            clabel = q.get("cognitive_level", "") or ""
            dlabel = q.get("difficulty", "")
            print(f"  {quiz_id}:{q['id']:4s}  [{tlabel}][{dlabel}][{clabel}] {q.get('question', '')[:60]}")
        print()

    print("=" * 70)
    print("\n输入要组装的题目 ID（格式：quiz_id:qid），多个用逗号分隔")
    print("示例：quiz_chapter_01:q1, quiz_chapter_01:q3, retest_chapter_01_xxx:q2")
    print("或输入 'all' 选择全部题目")
    print()

    selection = input("选择题目：").strip()
    if not selection:
        print("已取消。")
        return

    if selection.lower() == "all":
        qids = [f"{q['_quiz_id']}:{q['id']}" for q in all_qs]
    else:
        qids = [s.strip() for s in selection.split(",") if s.strip()]

    title = input("试卷标题（留空=自动生成）：").strip()

    quiz = assemble_quiz(subject, lesson, qids, title, storage)
    if quiz is None:
        print("未能匹配任何题目，请检查 ID 格式。")
        sys.exit(1)

    print(f"\n✅ 组装试卷已生成：{quiz.id}")
    print(f"   标题：{quiz.title}")
    print(f"   题目数：{len(quiz.questions)}")
    print(f"   来源试卷：{', '.join(quiz.source_quizzes)}")
    print()
    print(f"   开始答题：python main.py --exam --subject {subject} --lesson {lesson} --quiz-id {quiz.id}")


def cmd_exam_review(args):
    """Mixed review exam — automatically invokes Claude Code CLI to process."""
    from src.course_service import svc
    from src.storage import Storage
    from src.config_loader import get_config
    from src.agent_runner import process_task_via_claude_cli

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)
    _ensure_lesson_exists(storage, subject, lesson)

    model = getattr(args, 'model', None) or cfg.provider_config().get("model", "")
    num = int(args.num) if args.num else None

    task_id = svc().queue_exam_generation(subject, lesson, num, model)

    # Automatically invoke Claude Code CLI
    success = process_task_via_claude_cli(task_id)

    if not success:
        print()
        print("   ╔══════════════════════════════════════════════════════╗")
        print("   ║  📋 自动生成未完成，请在 Claude Code 中说：         ║")
        print("   ║     「处理所有待处理的学习任务」                    ║")
        print("   ╚══════════════════════════════════════════════════════╝")


def cmd_wrong_list(args):
    """List wrong questions for a lesson."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)

    wrong = list_wrong_questions(subject, lesson, storage)
    if not wrong:
        print("该课时暂无错题！🎉")
        return

    print(f"错题列表：{subject} 第{lesson}课（共 {len(wrong)} 题）\n")
    for i, wq in enumerate(wrong, start=1):
        print(f"{i}. {wq.get('question', 'N/A')}")
        print(f"   你的答案：{wq.get('user_answer', 'N/A')}")
        print(f"   正确答案：{wq.get('correct_answer', 'N/A')}")
        print()


def cmd_fix_wrong(args):
    """Queue AI wrong-question analysis task."""
    from src.agent_runner import queue_fix_wrong_task
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)
    _ensure_lesson_exists(storage, subject, lesson)

    wrong_list = list_wrong_questions(subject, lesson, storage)
    if not wrong_list:
        print("该课时暂无错题！🎉")
        return

    model = getattr(args, 'model', None) or cfg.provider_config().get("model", "")
    # Gather course content for context
    course_content = ""
    student_path = storage.student_notes_path(subject, lesson)
    if student_path.exists():
        course_content = storage.read_text(student_path)

    # Queue one task per wrong question (batch them)
    for i, wq in enumerate(wrong_list):
        output_dir = storage.wrong_questions_dir(subject, lesson)
        task_id = queue_fix_wrong_task(
            subject, lesson, wq, course_content, output_dir, model,
        )
        print(f"  [{i+1}/{len(wrong_list)}] 错题解析任务已排队：{task_id}")

    print()
    from src.agent_runner import process_all_pending_via_claude
    succeeded, failed = process_all_pending_via_claude()

    if failed > 0:
        print()
        print("   ╔══════════════════════════════════════════════════════╗")
        print("   ║  📋 部分任务未完成，请在 Claude Code 中说：         ║")
        print("   ║     「处理所有待处理的学习任务」                    ║")
        print("   ╚══════════════════════════════════════════════════════╝")


def cmd_process_tasks(args):
    """Process all pending tasks synchronously using direct AI API."""
    from src.agent_runner import process_pending_tasks_now, list_pending_tasks

    pending = list_pending_tasks()
    if not pending:
        print("✅ 没有待处理的任务。")
        return

    print(f"\n📋 待处理任务：{len(pending)} 个\n")
    for i, task in enumerate(pending, 1):
        print(f"  {i}. [{task.get('task_type', '?')}] {task.get('subject', '')} · {task.get('name', '')}")
        print(f"     ID: {task['task_id']}")

    print(f"\n⏳ 正在处理...\n")
    succeeded, failed = process_pending_tasks_now()
    print(f"\n✅ 处理完毕：成功 {succeeded} 个，失败 {failed} 个")


def cmd_score(args):
    """Show historical scores."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)

    scores = get_scores(subject, lesson, storage)
    if not scores:
        print("暂无考试记录。")
        return

    print(f"历史成绩：{subject} 第{lesson}课\n")
    for s in scores:
        pct = (s["score"] / s["total"] * 100) if s["total"] > 0 else 0
        print(f"  {s['timestamp']}：{s['score']}/{s['total']}（{pct:.0f}%）")


def cmd_summarize(args):
    """Queue AI summary evaluation task."""
    from src.agent_runner import queue_summarize_task
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)
    student_input = args.input

    if not student_input:
        print("错误：请通过 --input 提供你的总结文本。")
        sys.exit(1)

    _ensure_lesson_exists(storage, subject, lesson)

    model = getattr(args, 'model', None) or cfg.provider_config().get("model", "")

    course_content = ""
    student_path = storage.student_notes_path(subject, lesson)
    if student_path.exists():
        course_content = storage.read_text(student_path)

    output_dir = storage.lesson_dir(subject, lesson)
    task_id = queue_summarize_task(
        subject, lesson, student_input, course_content, output_dir, model,
    )

    print(f"✅ 评估任务已排队：{task_id}")
    print()

    from src.agent_runner import process_all_pending_via_claude
    succeeded, failed = process_all_pending_via_claude()

    if failed > 0:
        print()
        print("   ╔══════════════════════════════════════════════════════╗")
        print("   ║  📋 部分任务未完成，请在 Claude Code 中说：         ║")
        print("   ║     「处理所有待处理的学习任务」                    ║")
        print("   ╚══════════════════════════════════════════════════════╝")


def cmd_export_md(args):
    """Export merged notes as Markdown."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)
    _ensure_lesson_exists(storage, subject, lesson)

    export_merged_md(subject, lesson, storage)
    export_path = storage.lesson_dir(subject, lesson) / "export.md"
    print(f"✅ 已导出合并 MD：{export_path}")


def cmd_delete_subject(args):
    """Delete a subject by ID number or name."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subjects = storage.list_subjects()
    if not subjects:
        print("暂无科目。")
        return

    target = args.subject

    # Try numeric ID first
    try:
        idx = int(target) - 1
        if 0 <= idx < len(subjects):
            target = subjects[idx]
        else:
            print(f"无效编号：{target}")
            return
    except ValueError:
        pass  # treat as name

    lessons = storage.list_lessons(target)
    subject_dir = storage.subject_dir(target)
    if not lessons and not (subject_dir.exists() and (subject_dir / ".subject").exists()):
        print(f"科目「{target}」不存在。")
        return

    print(f"⚠️  将删除科目「{target}」及其全部 {len(lessons)} 个课时，不可恢复！")
    if input("确认删除？输入科目名确认: ").strip() == target:
        storage.delete_subject(target)
        print(f"✅ 已删除科目：{target}")
    else:
        print("已取消。")


def cmd_delete_lesson(args):
    """Delete a lesson by ID number or name."""
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson_str = args.lesson

    # If subject is a number, try to resolve from list
    subjects = storage.list_subjects()
    if subjects:
        try:
            idx = int(subject) - 1
            if 0 <= idx < len(subjects):
                subject = subjects[idx]
        except ValueError:
            pass  # treat as name

    lessons = storage.list_lessons(subject)
    if not lessons:
        print(f"科目「{subject}」不存在或没有课时。")
        return

    # Try numeric lesson ID
    lesson = None
    try:
        lid = int(lesson_str)
        if lid in lessons:
            lesson = lid
    except ValueError:
        pass

    if lesson is None:
        print(f"课时 {lesson_str} 不存在。可用课时：{', '.join(str(l) for l in sorted(lessons))}")
        return

    print(f"⚠️  将删除 {subject} 第{lesson}课，不可恢复！")
    if input("确认删除？(y/n): ").strip().lower() in ("y", "yes"):
        storage.delete_lesson(subject, lesson)
        print(f"✅ 已删除：{subject} 第{lesson}课")
    else:
        print("已取消。")


def cmd_fix_doc(args):
    """Queue note regeneration task."""
    from src.agent_runner import queue_regenerate_task
    from src.storage import Storage
    from src.config_loader import get_config

    cfg = get_config()
    storage = Storage(cfg.course_path)
    subject = args.subject
    lesson = int(args.lesson)
    _ensure_lesson_exists(storage, subject, lesson)

    model = getattr(args, 'model', None) or cfg.provider_config().get("model", "")

    source_text = ""
    raw_path = storage.raw_text_path(subject, lesson)
    if raw_path.exists():
        source_text = storage.read_text(raw_path)
    else:
        student_path = storage.student_notes_path(subject, lesson)
        if student_path.exists():
            source_text = storage.read_text(student_path)

    name = f"第{lesson}课"
    info_path = storage.info_path(subject, lesson)
    if info_path.exists():
        info_data = storage.read_json(info_path)
        name = info_data.get("name", name)

    output_dir = storage.lesson_dir(subject, lesson)
    task_id = queue_regenerate_task(subject, lesson, name, source_text, output_dir, model)

    print(f"✅ 重新生成任务已排队：{task_id}")
    print()

    from src.agent_runner import process_all_pending_via_claude
    succeeded, failed = process_all_pending_via_claude()

    if failed > 0:
        print()
        print("   ╔══════════════════════════════════════════════════════╗")
        print("   ║  📋 部分任务未完成，请在 Claude Code 中说：         ║")
        print("   ║     「处理所有待处理的学习任务」                    ║")
        print("   ╚══════════════════════════════════════════════════════╝")


# ---------------------------------------------------------------------------
# CLI setup
# ---------------------------------------------------------------------------

def main():
    try:
        _main()
    except (ConfigError, StorageError) as e:
        print(f"错误：{e}")
        sys.exit(1)
    except FileNotFoundError as e:
        print(f"错误：文件未找到 - {e}")
        sys.exit(1)
    except ValueError as e:
        print(f"参数错误：{e}")
        sys.exit(1)


def _main():
    parser = argparse.ArgumentParser(
        description="AI 多模态课程提取工具 — OpenAI 兼容 API 驱动（非交互）",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例：
  # 初始化 & 配置
  python main.py --init

  # 导入素材（非交互，直接生成笔记）
  python main.py --load-url https://www.bilibili.com/video/BVxxx --subject "高数" --lesson 1 --name "极限基础"
  python main.py --load-url https://www.bilibili.com/video/BVxxx --subject "高数" --lesson 1 --name "极限基础" --model deepseek-v4-pro
  python main.py --load-video demo.mp4 --load-doc book.pdf --subject "高数" --lesson 2 --name "导数"
  python main.py --source-text "导数是瞬时变化率" --subject "高数" --lesson 3 --name "导数定义"

  # 查看
  python main.py --subject-list
  python main.py --lessons --subject "高数"
  python main.py --learn --subject "高数" --lesson 1

  # 考试
  python main.py --exam-new --subject "高数" --lesson 1 --num 10
  python main.py --exam --subject "高数" --lesson 1
  python main.py --wrong-list --subject "高数" --lesson 1
  python main.py --fix-wrong --subject "高数" --lesson 1
  python main.py --score --subject "高数" --lesson 1

  # 总结 & 导出
  python main.py --summarize --subject "高数" --lesson 1 --input "极限就是无限趋近"
  python main.py --export-md --subject "高数" --lesson 1
  python main.py --fix-doc --subject "高数" --lesson 1

配置方式（优先级由高到低）：
  1. 命令行 --model 参数
  2. 环境变量 OPENAI_API_KEY / OPENAI_BASE_URL / OPENAI_MODEL
  3. config.json 中的配置
        """,
    )

    # --- Init ---
    parser.add_argument("--init", action="store_true", help="初始化项目目录")

    # --- Import ---
    parser.add_argument("--load-url", type=str, help="B站/YouTube 视频链接")
    parser.add_argument("--load-video", type=str, help="本地视频/音频文件路径")
    parser.add_argument("--load-doc", type=str, nargs="*", help="本地文档（PDF/Word/TXT/MD）")
    parser.add_argument("--source-text", type=str, help="直接输入的文本内容")

    # --- Common ---
    parser.add_argument("--subject", type=str, help="科目名称")
    parser.add_argument("--lesson", type=str, help="课时编号")
    parser.add_argument("--name", type=str, help="课时名称")
    parser.add_argument("--model", type=str, help="模型名称（覆盖 config.json，如 deepseek-v4-pro）")
    parser.add_argument("--raw", action="store_true", help="同时显示原始素材")
    parser.add_argument("--input", type=str, help="输入文本（用于 --summarize）")
    parser.add_argument("--num", type=str, help="题目数量（用于 --exam-new，留空自动策略决定）")
    parser.add_argument("--quiz-id", type=str, help="指定试卷 ID（用于 --exam）")
    parser.add_argument("--old-lesson", type=str, help="混合旧课时的错题（用于 --exam-review）")
    parser.add_argument("--ai-provider", type=str, help="AI 服务商（默认从 config.json 读取）")

    # --- Query ---
    parser.add_argument("--subject-list", action="store_true", help="查看所有科目")
    parser.add_argument("--lessons", action="store_true", help="查看科目下的课时")
    parser.add_argument("--learn", action="store_true", help="查看学习笔记")

    # --- Exam ---
    parser.add_argument("--exam-new", action="store_true", help="AI 生成新试卷")
    parser.add_argument("--exam", action="store_true", help="终端交互答题（可选 --quiz-id 指定历史试卷）")
    parser.add_argument("--exam-list", action="store_true", help="查看历史试卷列表")
    parser.add_argument("--exam-assemble", action="store_true", help="手动组装试卷（从历史试卷选题）")
    parser.add_argument("--exam-review", action="store_true", help="混合旧题复习考试")
    parser.add_argument("--wrong-list", action="store_true", help="查看错题列表")
    parser.add_argument("--fix-wrong", action="store_true", help="AI 生成错题解析")
    parser.add_argument("--score", action="store_true", help="查看历史分数")

    # --- Summary ---
    parser.add_argument("--summarize", action="store_true", help="AI 评估学习总结")

    # --- Docs ---
    parser.add_argument("--export-md", action="store_true", help="导出合并 Markdown")
    parser.add_argument("--fix-doc", action="store_true", help="AI 重新生成笔记")

    # --- Delete ---
    parser.add_argument("--delete-subject", action="store_true", help="删除整个科目")
    parser.add_argument("--delete-lesson", action="store_true", help="删除指定课时")

    # --- Tasks ---
    parser.add_argument("--process-tasks", action="store_true", help="同步处理所有待处理任务")

    args = parser.parse_args()

    # Dispatch
    if args.init:
        cmd_init(args)
    elif args.load_url:
        if not args.subject or not args.lesson:
            print("错误：--load-url 需要同时指定 --subject 和 --lesson。")
            sys.exit(1)
        cmd_load_url(args)
    elif args.load_video or args.load_doc:
        if not args.subject or not args.lesson:
            print("错误：--load-video/--load-doc 需要同时指定 --subject 和 --lesson。")
            sys.exit(1)
        cmd_load_video(args)
    elif args.source_text:
        if not args.subject or not args.lesson:
            print("错误：--source-text 需要同时指定 --subject 和 --lesson。")
            sys.exit(1)
        cmd_load_source_text(args)
    elif args.subject_list:
        cmd_subject_list(args)
    elif args.lessons:
        if not args.subject:
            print("错误：--lessons 需要 --subject。")
            sys.exit(1)
        cmd_lessons(args)
    elif args.learn:
        if not args.subject or not args.lesson:
            print("错误：--learn 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_learn(args)
    elif args.exam_new:
        if not args.subject or not args.lesson:
            print("错误：--exam-new 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_exam_new(args)
    elif args.exam:
        if not args.subject or not args.lesson:
            print("错误：--exam 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_exam(args)
    elif args.exam_list:
        if not args.subject or not args.lesson:
            print("错误：--exam-list 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_exam_list(args)
    elif args.exam_assemble:
        if not args.subject or not args.lesson:
            print("错误：--exam-assemble 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_exam_assemble(args)
    elif args.exam_review:
        if not args.subject or not args.lesson:
            print("错误：--exam-review 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_exam_review(args)
    elif args.wrong_list:
        if not args.subject or not args.lesson:
            print("错误：--wrong-list 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_wrong_list(args)
    elif args.fix_wrong:
        if not args.subject or not args.lesson:
            print("错误：--fix-wrong 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_fix_wrong(args)
    elif args.score:
        if not args.subject or not args.lesson:
            print("错误：--score 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_score(args)
    elif args.summarize:
        if not args.subject or not args.lesson:
            print("错误：--summarize 需要 --subject、--lesson 和 --input。")
            sys.exit(1)
        cmd_summarize(args)
    elif args.export_md:
        if not args.subject or not args.lesson:
            print("错误：--export-md 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_export_md(args)
    elif args.fix_doc:
        if not args.subject or not args.lesson:
            print("错误：--fix-doc 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_fix_doc(args)
    elif args.delete_subject:
        if not args.subject:
            print("错误：--delete-subject 需要 --subject。")
            sys.exit(1)
        cmd_delete_subject(args)
    elif args.delete_lesson:
        if not args.subject or not args.lesson:
            print("错误：--delete-lesson 需要 --subject 和 --lesson。")
            sys.exit(1)
        cmd_delete_lesson(args)
    elif args.process_tasks:
        cmd_process_tasks(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
