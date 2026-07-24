"""前端 — 多级菜单交互界面

纯展示 + 用户输入，所有业务逻辑委托给 src/course_service.py。
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(PROJECT_ROOT))

from dotenv import load_dotenv
load_dotenv()

from src.config_loader import (
    CLAUDE_AGENT_COMMAND,
    CODEX_AGENT_COMMAND,
    DEFAULT_AGENT_COMMAND,
    get_config,
    resolve_agent_command,
)
from src.course_service import CourseService, svc

# ── Colors ──
B = "\033[1m"; D = "\033[2m"; R = "\033[0m"
Rd = "\033[31m"; Gn = "\033[32m"; Yw = "\033[33m"; Cy = "\033[36m"

# ── Helpers ──

BACK_COMMANDS = {"0", "q", "quit", "exit", "b", "back", "返回", "退出", "取消"}

def clear():
    os.system("clear 2>/dev/null || true")

def banner(subtitle: str = ""):
    clear()
    print(f"\n{Cy}{B}  ╔══════════════════════════════════════╗{R}")
    print(f"{Cy}{B}  ║     AI 学习助教                          ║{R}")
    print(f"{Cy}{B}  ╚══════════════════════════════════════╝{R}")
    if subtitle:
        print(f"  {B}{subtitle}{R}")
    print()

def is_back_command(value: str) -> bool:
    return value.strip().lower() in BACK_COMMANDS

def ask(prompt: str, default: str = "", allow_back: bool = False) -> str | None:
    d = f" {D}[{default}]{R}" if default else ""
    back = f" {D}[0/q 返回]{R}" if allow_back else ""
    try:
        value = input(f"  {Gn}?{R} {prompt}{d}{back}: ").strip()
    except (EOFError, KeyboardInterrupt):
        print(); return None if allow_back else ""
    if allow_back and is_back_command(value):
        return None
    return value or default

def default_agent_command() -> str:
    try:
        return get_config().agent_command
    except Exception:
        return DEFAULT_AGENT_COMMAND


def _agent_label(command: str) -> str:
    if command == CODEX_AGENT_COMMAND:
        return "Codex"
    if command == CLAUDE_AGENT_COMMAND:
        return "Claude Code"
    return "自定义"


def ask_agent_command(allow_back: bool = True) -> str | None:
    default = default_agent_command()
    print(f"\n  {B}选择 AI Agent：{R}")
    for key, label, command in (
        ("1", "Codex", CODEX_AGENT_COMMAND),
        ("2", "Claude Code", CLAUDE_AGENT_COMMAND),
    ):
        status = f" {D}(默认){R}" if command == default else ""
        print(f"    {Gn}{key}{R}  {label}{status}")
        print(f"       {D}{command}{R}")
    print(f"    {Gn}C{R}  自定义命令")

    back = f" {D}[0/q 返回]{R}" if allow_back else ""
    prompt = f"  {Gn}?{R} AI Agent {D}[默认: {_agent_label(default)}]{R}{back}: "
    try:
        value = input(prompt).strip()
    except (EOFError, KeyboardInterrupt):
        print(); return None if allow_back else ""
    if allow_back and is_back_command(value):
        return None
    if not value:
        return default

    key = value.lower()
    if key in {"1", "codex"}:
        return CODEX_AGENT_COMMAND
    if key in {"2", "claude", "claude code"}:
        return CLAUDE_AGENT_COMMAND
    if key in {"c", "custom", "自定义"}:
        return ask("AI 命令", default, allow_back=allow_back)
    return resolve_agent_command(value)

def confirm(prompt: str, default: bool = True) -> bool:
    yn = "Y/n" if default else "y/N"
    try:
        r = input(f"  {Yw}⚠{R}  {prompt} {D}[{yn}]{R}: ").strip().lower()
    except (EOFError, KeyboardInterrupt):
        print(); return False
    return r in ("y", "yes") if r else default

def show(s=""):       print(f"  {s}")
def ok(s=""):         print(f"\n  {Gn}✅ {s}{R}")
def warn(s=""):       print(f"\n  {Yw}⚠️  {s}{R}")
def err(s=""):        print(f"\n  {Rd}❌ {s}{R}")
def sep():            print(f"  {D}{'─' * 44}{R}")
def hint(s=""):       print(f"  {D}{s}{R}")

def show_source_file(path: Path):
    show(f"{D}原始文件：{path.resolve()}{R}")
    show(f"{D}可直接用电脑打开这个 .md 文件。{R}")

def read_multiline_until_end(prompt: str) -> str | None:
    print(f"  {Yw}{prompt}（单独一行 END 结束，0/q 返回）：{R}")
    lines = []
    while True:
        try:
            line = input()
        except (EOFError, KeyboardInterrupt):
            print(); return None
        if is_back_command(line):
            return None
        if line.strip().upper() == "END":
            break
        lines.append(line)
    return "\n".join(lines)

def wait():
    try:
        input(f"\n  {D}按 Enter 继续...{R}")
    except (EOFError, KeyboardInterrupt):
        print()

def mi(num, label, status=""):
    """Print a menu item."""
    s = f"  {Gn}{str(num):>2}{R}  {label}"
    if status:
        s += f"  {D}{status}{R}"
    print(s)

# ── Pickers ──

def pick_subject(svc: CourseService, prompt: str = "选择科目") -> str | None:
    subs = svc.list_subjects()
    if not subs:
        show(f"{D}暂无科目，请先创建。{R}")
        return None
    print(f"\n  {B}{prompt}：{R}")
    for i, s in enumerate(subs, 1):
        cnt = len(svc.list_lessons(s))
        print(f"    {Gn}{i}{R}  {s} {D}({cnt} 课时){R}")
    print(f"    {Gn}0{R}  返回")
    print()
    ch = ask("输入编号或名称", "1", allow_back=True)
    if ch is None or not ch:
        return None
    try:
        idx = int(ch) - 1
        if 0 <= idx < len(subs):
            return subs[idx]
    except ValueError:
        pass
    for s in subs:
        if ch in s or s.lower() == ch.lower():
            return s
    err(f"未找到：{ch}")
    return None

def pick_lesson(svc: CourseService, subj: str) -> int | None:
    lessons = svc.list_lessons(subj)
    if not lessons:
        show(f"{D}「{subj}」暂无课时。{R}")
        return None
    print(f"\n  {B}课时列表 — {subj}：{R}")
    for ln in lessons:
        status = svc.lesson_status(subj, ln)
        flags = ""
        if status["teacher"]: flags += "📖"
        if status["student"]: flags += "📝"
        if status["quiz"]:    flags += "📋"
        wc = svc.wrong_count(subj, ln)
        if wc: flags += f" {Rd}✗{wc}{R}"
        print(f"    {Gn}{ln}{R}  {svc.lesson_name(subj, ln)}  {D}{flags}{R}")
    print(f"    {Gn}0{R}  返回")
    print()
    ch = ask("课时编号", str(lessons[-1]), allow_back=True)
    if ch is None:
        return None
    try:
        ln = int(ch)
        if ln in lessons:
            return ln
    except ValueError:
        pass
    err(f"课时 {ch} 不存在")
    return None

# ═══════════════════════════════════════════════════════════════
#  Level 1 — 科目列表
# ═══════════════════════════════════════════════════════════════

def menu_main(svc: CourseService):
    while True:
        banner("科目列表")
        subs = svc.list_subjects()

        # Pending tasks indicator
        from .agent_runner import get_pending_count
        pending = get_pending_count()
        if pending:
            show(f"{Yw}📋 {pending} 个待处理 AI 任务 — 在 Claude Code 中请求处理{R}\n")

        if not subs:
            show(f"{D}暂无科目，请先创建。{R}\n")

        for i, s in enumerate(subs, 1):
            cnt = len(svc.list_lessons(s))
            mi(i, s, f"({cnt} 课时)")

        sep()
        mi("N", "新建科目")
        if subs:
            mi("D", "删除科目")
        sep()
        mi(0, "退出")
        print()

        ch = ask("输入编号进入科目", "0").upper()
        if ch in ("0", ""):
            print(f"\n  {Gn}再见！{R}\n"); break
        if ch == "N":
            _create_subject(svc)
            continue
        if ch == "D" and subs:
            _delete_subject(svc, subs)
            continue

        try:
            idx = int(ch) - 1
            if 0 <= idx < len(subs):
                menu_subject(svc, subs[idx])
            else:
                err("无效编号")
        except ValueError:
            err("无效输入")

# ═══════════════════════════════════════════════════════════════
#  Level 2 — 科目内部
# ═══════════════════════════════════════════════════════════════

def menu_subject(svc: CourseService, subj: str):
    while True:
        banner(f"📚 {subj}")
        lessons = svc.list_lessons(subj)

        if not lessons:
            show(f"{D}暂无课时，请先导入。{R}\n")
        else:
            # ── 课时列表 ──
            show(f"{B}课时列表：{R}")
            for ln in lessons:
                status = svc.lesson_status(subj, ln)
                flags = ""
                if status["teacher"]: flags += "📖"
                if status["student"]: flags += "📝"
                if status["quiz"]:    flags += "📋"
                wc = svc.wrong_count(subj, ln)
                if wc: flags += f" {Rd}✗{wc}{R}"
                mi(ln, svc.lesson_name(subj, ln), flags)
            print()

        # ── 学生区 ──
        show(f"{Gn}{B}🎓 学生区{R}")
        if lessons:
            mi("VS", "查看学生笔记")
            mi("ES", "进入考场（答题 / 错题 / 成绩）")
            mi("SS", "AI 评估我的学习总结")
        else:
            hint("  导入课时后可用")

        # ── 教师区 ──
        print()
        show(f"{Cy}{B}📖 教师区{R}")
        if lessons:
            mi("VT", "查看教师教纲")
            mi("ET", "管理考试（出题 / 复习 / 解析）")
        mi("A",  "导入新课時（AI 生成教纲 + 笔记）")
        if lessons:
            mi("X",  "导出 / 重新生成笔记")
            mi("D",  "删除课时")
        print()

        sep()
        mi(0, "返回主菜单")
        print()

        ch = ask("输入编号+字母（如 1VS 看笔记, 1ES 进考场）").strip()
        if not ch:
            continue
        upper = ch.upper()

        if upper == "0":
            break
        if upper == "A":
            _add_lesson(svc, subj); continue
        if upper == "D" and lessons:
            _delete_lesson(svc, subj, lessons); continue
        if upper == "X" and lessons:
            _pick_lesson_then(svc, subj, lessons, lambda s, l: _doc_ops(s, l)); continue

        # Parse lesson number + action
        lesson_part = ""
        action = ""
        for c in ch:
            if c.isdigit():
                lesson_part += c
            else:
                action += c.upper()

        if not lesson_part:
            if action in ("VS", "VT", "ES", "ET", "SS"):
                err("请先输入课时编号。例如：1VS 查看第1课学生笔记")
            continue

        ln = int(lesson_part)
        if ln not in lessons:
            err(f"课时 {ln} 不存在"); continue

        # Route
        if action in ("VS", ""):
            _view_student_notes(svc, subj, ln)
        elif action == "VT":
            _view_teacher_notes(svc, subj, ln)
        elif action == "ES":
            menu_exam(svc, subj, ln)
        elif action == "ET":
            _teacher_exam(svc, subj, ln)
        elif action == "SS":
            _summarize(svc, subj, ln)
        else:
            _view_student_notes(svc, subj, ln)

def _pick_lesson_then(svc, subj, lessons, fn):
    """If no lesson number in input, show picker first."""
    ln = pick_lesson(svc, subj)
    if ln:
        fn(svc, subj, ln)

# ═══════════════════════════════════════════════════════════════
#  Level 3 — 考试中心
# ═══════════════════════════════════════════════════════════════

def menu_exam(svc: CourseService, subj: str, ln: int):
    while True:
        banner(f"📝 考试中心 → {subj} · {svc.lesson_name(subj, ln)}")
        status = svc.lesson_status(subj, ln)
        wc = svc.wrong_count(subj, ln)
        from src.exam_logic import list_pending_grading_attempts
        pending_grading = list_pending_grading_attempts(subj, ln, svc.storage)
        all_grading_attempts = list_pending_grading_attempts(
            subj, ln, svc.storage, include_completed=True
        )

        mi(1, "生成新试卷",    f"{Gn}已生成{R}" if status["quiz"] else f"{D}未生成{R}")
        mi(2, "开始答题",      "")
        mi(3, "历史试卷",      "")
        mi(4, "组装试卷",      "人工从历史试卷选题")
        mi(5, "错题复习考试",  "")
        mi(6, "查看/解析错题", f"{Rd}{wc} 道{R}" if wc else f"{D}无{R}")
        mi(7, "查看成绩",      "")
        grading_status = f"{Rd}{len(pending_grading)} 条{R}" if pending_grading else ""
        if not grading_status and all_grading_attempts:
            grading_status = f"{Cy}可重评 {len(all_grading_attempts)} 条{R}"
        mi(8, "待点评/重试AI点评", grading_status if grading_status else f"{D}无{R}")
        sep(); mi(0, "返回科目"); print()

        ch = ask("选择")
        if ch == "0": break
        elif ch == "1": _exam_new(svc, subj, ln)
        elif ch == "2":
            if not status["quiz"]: warn("请先生成试卷。"); wait(); continue
            _exam_take(svc, subj, ln)
        elif ch == "3": _exam_list(svc, subj, ln)
        elif ch == "4": _exam_assemble(svc, subj, ln)
        elif ch == "5": _exam_review(svc, subj, ln)
        elif ch == "6": _exam_wrong(svc, subj, ln)
        elif ch == "7": _exam_score(svc, subj, ln)
        elif ch == "8": _exam_grading_pending(svc, subj, ln)

# ═══════════════════════════════════════════════════════════════
#  Actions (call service layer)
# ═══════════════════════════════════════════════════════════════

def _create_subject(svc: CourseService):
    banner("📁 新建科目")
    subs = svc.list_subjects()
    if subs:
        show("已有科目：" + "、".join(subs))
        print()
    name = ask("新科目名称（如：高等数学）", allow_back=True)
    if name is None:
        return
    if not name:
        err("名称不能为空。"); wait(); return
    try:
        svc.create_subject(name)
        ok(f"科目「{name}」创建成功。")
    except Exception as e:
        err(str(e))
    wait()

def _delete_subject(svc: CourseService, subs: list):
    banner("🗑️  删除科目")
    for i, s in enumerate(subs, 1):
        show(f"  {i}  {s}")
    show(f"  0  返回")
    ch = ask("科目编号", allow_back=True)
    if ch is None:
        return
    try:
        idx = int(ch) - 1
        if 0 <= idx < len(subs):
            s = subs[idx]
            if confirm(f"删除「{s}」？{Rd}不可恢复！{R}", default=False):
                svc.delete_subject(s)
                ok("已删除。"); wait()
    except ValueError:
        err("无效编号"); wait()

def _add_lesson(svc: CourseService, subj: str):
    banner(f"📥 导入课时 → {subj}")
    existing = svc.list_lessons(subj)
    if existing:
        show(f"已有课时：{', '.join(str(l) for l in existing)}")
    default_ln = str(max(existing) + 1) if existing else "1"
    ln_str = ask("课时编号", default_ln, allow_back=True)
    if ln_str is None:
        return
    ln = int(ln_str) if ln_str.isdigit() else 1
    name = ask("课时名称", f"第{ln}课", allow_back=True)
    if name is None:
        return

    print(f"\n  {B}素材来源：{R}")
    print(f"    {Gn}1{R}  视频链接（YouTube / B站）← Claude Code 自动获取")
    print(f"    {Gn}2{R}  本地文件")
    print(f"    {Gn}3{R}  直接粘贴文本")
    print(f"    {Gn}0{R}  返回")
    st = ask("选择", "1", allow_back=True)
    if st is None:
        return

    flag, value = "", ""
    if st == "1":
        source = ask("视频链接", allow_back=True)
        if source is None:
            return
        flag, value = "--load-url", source
    elif st == "2":
        source = ask("文件路径", allow_back=True)
        if source is None:
            return
        flag, value = "--load-video", source
    elif st == "3":
        flag, value = "--source-text", ""
        value = read_multiline_until_end("输入文本")
        if value is None:
            return
        if not value.strip():
            err("内容不能为空。"); wait(); return
        hint(f"已输入 {len(value)} 字符")
    else:
        err("无效选择。"); wait(); return

    agent_command = ask_agent_command()
    if agent_command is None:
        return

    # Confirm
    print(f"\n  {B}{'─' * 44}{R}")
    show(f"科目：{Gn}{subj}{R}")
    show(f"课时：{Gn}第{ln}课 · {name}{R}")
    show(f"素材：{Gn}{value[:70]}{'...' if len(value) > 70 else ''}{R}")
    show(f"命令：{Gn}{agent_command}{R}")
    sep()
    if not confirm("确认排队导入任务？"):
        show("已取消。"); wait(); return

    task_id = svc.queue_import_lesson(subj, ln, name, flag, value, agent_command=agent_command)
    ok(f"任务已排队（{task_id}）")

    from src.agent_runner import process_all_pending_via_claude
    succeeded, failed = process_all_pending_via_claude(agent_command=agent_command)

    if succeeded > 0:
        ok(f"处理完毕！成功 {succeeded} 个任务。按 {Gn}{ln}VS{R} 查看学生笔记。")
    else:
        warn("自动处理未完成，任务已保存。")
        show(f"{Cy}请在 Claude Code 中说「处理所有待处理的学习任务」{R}")
    wait()

def _view_student_notes(svc: CourseService, subj: str, ln: int):
    """学生视图：只看学生笔记。"""
    banner(f"🎓 学生笔记 → {subj} · {svc.lesson_name(subj, ln)}")
    student_path = svc.storage.student_notes_path(subj, ln)
    if student_path.exists():
        show_source_file(student_path)
        sep()
        print(svc.storage.read_text(student_path))
    else:
        warn("学生笔记尚未生成。请先导入课时。")
    wait()

def _view_teacher_notes(svc: CourseService, subj: str, ln: int):
    """教师视图：只看教师教纲。"""
    banner(f"📖 教师教纲 → {subj} · {svc.lesson_name(subj, ln)}")
    teacher_path = svc.storage.teacher_notes_path(subj, ln)
    if teacher_path.exists():
        show_source_file(teacher_path)
        sep()
        print(svc.storage.read_text(teacher_path))
    else:
        warn("教师教纲尚未生成。请先导入课时。")
    wait()

def _teacher_exam(svc: CourseService, subj: str, ln: int):
    """教师考试管理：出题、复习、查看解析。"""
    while True:
        banner(f"📖 考试管理 → {subj} · {svc.lesson_name(subj, ln)}")
        status = svc.lesson_status(subj, ln)
        wc = svc.wrong_count(subj, ln)

        mi(1, "生成新试卷",      f"{Gn}已生成{R}" if status["quiz"] else f"{D}未生成{R}")
        mi(2, "历史试卷",        "")
        mi(3, "组装试卷",        "人工从历史试卷选题")
        mi(4, "错题复习考试",    f"{Rd}{wc} 道错题{R}" if wc else "")
        mi(5, "查看/解析错题",   f"{Rd}{wc} 道{R}" if wc else f"{D}无{R}")
        mi(6, "查看成绩",        "")
        sep(); mi(0, "返回科目"); print()

        ch = ask("选择")
        if ch == "0": break
        elif ch == "1": _exam_new(svc, subj, ln)
        elif ch == "2": _exam_list(svc, subj, ln)
        elif ch == "3": _exam_assemble(svc, subj, ln)
        elif ch == "4":
            if wc == 0: warn("暂无错题。"); wait(); continue
            _exam_review(svc, subj, ln)
        elif ch == "5": _exam_wrong(svc, subj, ln)
        elif ch == "6": _exam_score(svc, subj, ln)

def _delete_lesson(svc: CourseService, subj: str, lessons: list):
    banner(f"🗑️  删除课时 → {subj}")
    for ln in lessons:
        show(f"  {ln}  {svc.lesson_name(subj, ln)}")
    show(f"  0  返回")
    ch = ask("课时编号", allow_back=True)
    if ch is None:
        return
    try:
        ln = int(ch)
        if ln in lessons:
            if confirm(f"删除 {subj} · {svc.lesson_name(subj, ln)}？{Rd}不可恢复！{R}", default=False):
                svc.delete_lesson(subj, ln)
                ok("已删除。"); wait()
    except ValueError:
        err("无效编号"); wait()

def _summarize(svc: CourseService, subj: str, ln: int):
    banner(f"🤔 AI 评估总结 → {subj} · {svc.lesson_name(subj, ln)}")
    agent_command = ask_agent_command()
    if agent_command is None:
        return
    text = read_multiline_until_end("输入你的学习总结")
    if text is None:
        return
    if not text.strip():
        err("内容不能为空。"); wait(); return

    from .agent_runner import queue_summarize_task

    # Gather course content
    course_content = ""
    student_path = svc.storage.student_notes_path(subj, ln)
    if student_path.exists():
        course_content = svc.storage.read_text(student_path)

    output_dir = svc.storage.lesson_dir(subj, ln)
    task_id = queue_summarize_task(
        subj, ln, text, course_content, output_dir, agent_command=agent_command
    )
    ok(f"评估任务已排队（{task_id}）")

    from src.agent_runner import process_all_pending_via_claude
    succeeded, failed = process_all_pending_via_claude(agent_command=agent_command)

    if succeeded > 0:
        ok(f"处理完毕！成功 {succeeded} 个任务。")
    else:
        warn("自动处理未完成，任务已保存。")
        show(f"{Cy}请在 Claude Code 中说「处理所有待处理的学习任务」{R}")
    wait()

def _doc_ops(svc: CourseService, subj: str, ln: int):
    banner(f"📤 文档操作 → {subj} · {svc.lesson_name(subj, ln)}")
    print(f"    {Gn}1{R}  导出合并 Markdown")
    print(f"    {Gn}2{R}  AI 重新生成笔记（Claude Code Agent）")
    print(f"    {Gn}0{R}  返回")
    ch = ask("选择", "1", allow_back=True)
    if ch is None:
        return
    if ch == "1":
        import subprocess, sys
        subprocess.Popen([
            sys.executable, str(PROJECT_ROOT / "main.py"),
            "--export-md", "--subject", subj, "--lesson", str(ln),
        ]).wait()
    elif ch == "2":
        agent_command = ask_agent_command()
        if agent_command is None:
            return
        from .agent_runner import queue_regenerate_task

        # Gather source text
        source_text = ""
        raw_path = svc.storage.raw_text_path(subj, ln)
        if raw_path.exists():
            source_text = svc.storage.read_text(raw_path)
        else:
            student_path = svc.storage.student_notes_path(subj, ln)
            if student_path.exists():
                source_text = svc.storage.read_text(student_path)

        output_dir = svc.storage.lesson_dir(subj, ln)
        task_id = queue_regenerate_task(
            subj, ln, svc.lesson_name(subj, ln),
            source_text, output_dir, agent_command=agent_command,
        )
        ok(f"重新生成任务已排队（{task_id}）")

        from src.agent_runner import process_all_pending_via_claude
        succeeded, failed = process_all_pending_via_claude(agent_command=agent_command)

        if succeeded > 0:
            ok(f"处理完毕！成功 {succeeded} 个任务。按 {Gn}{ln}VS{R} 查看笔记。")
        else:
            warn("自动处理未完成，任务已保存。")
            show(f"{Cy}请在 Claude Code 中说「处理所有待处理的学习任务」{R}")
    wait()

def _exam_new(svc: CourseService, subj: str, ln: int):
    """Queue exam generation task — Claude Code CLI processes it automatically."""
    from src.agent_runner import process_task_via_claude_cli

    banner(f"📝 生成新试卷 → {subj} · {svc.lesson_name(subj, ln)}")
    num = ask("题目数量（留空=策略自动）", "", allow_back=True)
    if num is None:
        return
    try:
        num_int = int(num) if num.strip() else None
    except ValueError:
        warn("题目数量必须是数字。"); wait(); return
    agent_command = ask_agent_command()
    if agent_command is None:
        return

    task_id = svc.queue_exam_generation(subj, ln, num_int, agent_command=agent_command)

    # Automatically invoke Claude Code CLI to process the task
    success = process_task_via_claude_cli(task_id, agent_command=agent_command)

    if success:
        ok("试卷生成完毕！返回菜单后可开始答题。")
    else:
        warn("自动生成未完成，任务已保存。")
        show(f"{Cy}请在 Claude Code 中说「处理所有待处理的学习任务」{R}")

    wait()

def _exam_take(svc: CourseService, subj: str, ln: int):
    import subprocess, sys
    subprocess.Popen([
        sys.executable, str(PROJECT_ROOT / "main.py"),
        "--exam", "--subject", subj, "--lesson", str(ln),
    ]).wait()
    wait()

def _exam_review(svc: CourseService, subj: str, ln: int):
    """Queue review exam task — Claude Code CLI processes it automatically."""
    from src.agent_runner import process_task_via_claude_cli

    banner(f"🔄 错题复习考试 → {subj} · {svc.lesson_name(subj, ln)}")
    agent_command = ask_agent_command()
    if agent_command is None:
        return

    task_id = svc.queue_exam_generation(subj, ln, None, agent_command=agent_command)

    # Automatically invoke Claude Code CLI to process the task
    success = process_task_via_claude_cli(task_id, agent_command=agent_command)

    if success:
        ok("复习试卷生成完毕！")
    else:
        warn("自动生成未完成，任务已保存。")
        show(f"{Cy}请在 Claude Code 中说「处理所有待处理的学习任务」{R}")

    wait()

def _exam_wrong(svc: CourseService, subj: str, ln: int):
    banner(f"🧠 错题处理 → {subj} · {svc.lesson_name(subj, ln)}")
    wc = svc.wrong_count(subj, ln)
    show(f"当前 {Rd}{wc} 道错题{R}" if wc else f"{D}暂无错题 🎉{R}")
    print(f"\n  {Gn}1{R}  查看错题列表")
    print(f"  {Gn}2{R}  AI 生成错题解析")
    print(f"  {Gn}0{R}  返回")
    ch = ask("选择", "1", allow_back=True)
    if ch is None:
        return
    import subprocess, sys
    if ch == "1":
        subprocess.Popen([
            sys.executable, str(PROJECT_ROOT / "main.py"),
            "--wrong-list", "--subject", subj, "--lesson", str(ln),
        ]).wait()
    elif ch == "2":
        subprocess.Popen([
            sys.executable, str(PROJECT_ROOT / "main.py"),
            "--fix-wrong", "--subject", subj, "--lesson", str(ln),
        ]).wait()
    wait()

def _exam_score(svc: CourseService, subj: str, ln: int):
    import subprocess, sys
    subprocess.Popen([
        sys.executable, str(PROJECT_ROOT / "main.py"),
        "--score", "--subject", subj, "--lesson", str(ln),
    ]).wait()
    wait()


def _exam_grading_pending(svc: CourseService, subj: str, ln: int):
    from src.exam_logic import (
        grade_attempt_with_claude_cli,
        list_pending_grading_attempts,
        mark_grading_failed,
    )

    attempts = list_pending_grading_attempts(
        subj, ln, svc.storage, include_completed=True
    )
    if not attempts:
        ok("暂无待点评或点评失败记录。")
        wait()
        return

    banner(f"🧾 待点评记录 → {subj} · {svc.lesson_name(subj, ln)}")
    pending_attempts = [item for item in attempts if item["status"] != "completed"]
    completed_attempts = [item for item in attempts if item["status"] == "completed"]
    for i, item in enumerate(attempts, start=1):
        if item["status"] == "failed":
            status_label = "点评失败"
        elif item["status"] == "completed":
            status_label = "已点评，可重评"
        else:
            status_label = "未点评"
        show(f"{i}. [{status_label}] {item['quiz_id']}")
        show(f"   {D}{item['answers_path']}{R}")
        if item.get("error"):
            show(f"   失败原因：{item['error']}")
    print()
    choices: dict[str, list[dict]] = {}
    idx = 1
    if pending_attempts:
        choices[str(idx)] = pending_attempts
        print(f"  {Gn}{idx}{R}  重试待点评/失败记录")
        idx += 1
    if completed_attempts:
        choices[str(idx)] = completed_attempts
        print(f"  {Gn}{idx}{R}  重新点评已完成记录")
        idx += 1
    if pending_attempts and completed_attempts:
        choices[str(idx)] = attempts
        print(f"  {Gn}{idx}{R}  全部处理")
    print(f"  {Gn}0{R}  返回")
    ch = ask("选择", "1", allow_back=True)
    if ch is None or ch == "0":
        return
    selected = choices.get(ch)
    if selected is None:
        warn("无效选择。")
        wait()
        return
    agent_command = ask_agent_command()
    if agent_command is None:
        return

    succeeded = 0
    failed = 0
    for item in selected:
        try:
            success = grade_attempt_with_claude_cli(
                subj, ln, item["quiz_id"], item["answers_path"], svc.storage,
                agent_command=agent_command,
            )
            if success:
                succeeded += 1
                answer_record = svc.storage.read_json(Path(item["answers_path"]))
                feedback_md = answer_record.get("ai_grading", {}).get("feedback_md_path", "")
                ok(f"已点评：{item['quiz_id']}")
                if feedback_md:
                    show(f"点评文件：{Path(feedback_md).resolve()}")
            else:
                failed += 1
                mark_grading_failed(svc.storage, item["answers_path"], "Claude Code CLI 未完成试卷检测")
                warn(f"点评失败：{item['quiz_id']}：Claude Code CLI 未完成试卷检测")
        except Exception as e:
            failed += 1
            mark_grading_failed(svc.storage, item["answers_path"], e)
            warn(f"点评失败：{item['quiz_id']}：{e}")
    show(f"处理完成：成功 {succeeded} 条，失败 {failed} 条")
    wait()


def _exam_list(svc: CourseService, subj: str, ln: int):
    """Browse historical quizzes and select one to take."""
    from src.exam_logic import list_historical_quizzes, load_quiz_by_id

    quizzes = list_historical_quizzes(subj, ln, svc.storage)
    if not quizzes:
        warn("暂无历史试卷。"); wait(); return

    while True:
        banner(f"📋 历史试卷 → {subj} · {svc.lesson_name(subj, ln)}")
        src_label = {"regular": "📝常规", "retest": "🔄复习", "assembled": "🧩组装"}
        for i, qm in enumerate(quizzes, start=1):
            label = src_label.get(qm.get("source", ""), qm.get("source", ""))
            show(f"  {Gn}{i}{R}. {label} {qm['title']}")
            show(f"     ID: {D}{qm['id']}{R} | {qm['question_count']} 题 | {qm.get('created_at', '')[:10]}")
        print()
        mi(1, "选择试卷答题")
        mi(2, "查看试卷详情")
        sep(); mi(0, "返回"); print()

        ch = ask("选择")
        if ch == "0": break
        elif ch == "1":
            idx_s = ask("试卷编号", "1", allow_back=True)
            if idx_s is None:
                continue
            try:
                idx = int(idx_s) - 1
                if 0 <= idx < len(quizzes):
                    quiz = load_quiz_by_id(subj, ln, quizzes[idx]["id"], svc.storage)
                    if quiz:
                        _run_quiz(subj, ln, quiz)
                    else:
                        warn("加载试卷失败。"); wait()
            except (ValueError, IndexError):
                warn("无效编号。"); wait()
        elif ch == "2":
            idx_s = ask("试卷编号", "1", allow_back=True)
            if idx_s is None:
                continue
            try:
                idx = int(idx_s) - 1
                if 0 <= idx < len(quizzes):
                    _view_quiz_detail(subj, ln, quizzes[idx], svc)
            except (ValueError, IndexError):
                warn("无效编号。"); wait()


def _view_quiz_detail(subj: str, ln: int, qmeta: dict, svc: CourseService):
    """Show detailed view of a quiz."""
    from src.exam_logic import load_quiz_by_id
    quiz = load_quiz_by_id(subj, ln, qmeta["id"], svc.storage)
    if quiz is None:
        warn("无法加载试卷。"); wait(); return

    type_label = {
        "single_choice": "单选", "multiple_choice": "多选", "true_false": "判断",
        "short_answer": "简答", "case_analysis": "案例分析",
    }
    diff_emoji = {"easy": "🟢", "medium": "🟡", "hard": "🟠", "challenge": "🔴"}

    banner(f"📄 {quiz.title}")
    for i, q in enumerate(quiz.questions, start=1):
        tlabel = type_label.get(q.type.value if hasattr(q.type, 'value') else str(q.type), str(q.type))
        clabel = q.cognitive_level.value if q.cognitive_level else "?"
        dlabel = q.difficulty
        de = diff_emoji.get(dlabel, "")
        print(f"  {B}{i}.{R} [{tlabel}] {de}{dlabel} [{clabel}]")
        print(f"     {q.question[:80]}")
        if q.options:
            opts = ", ".join(f"{k}={v}" for k, v in list(q.options.items())[:4])
            print(f"     {D}选项：{opts}{R}")
        concepts = "、".join(q.tested_concepts[:3])
        print(f"     {D}概念：{concepts}{R}")
        print()
    print(f"  {D}共 {len(quiz.questions)} 题 | 来源：{quiz.source}{R}")
    wait()


def _exam_assemble(svc: CourseService, subj: str, ln: int):
    """Manually assemble a quiz by picking questions from history."""
    from src.exam_logic import list_all_questions, list_historical_quizzes, assemble_quiz

    all_qs = list_all_questions(subj, ln, svc.storage)
    if not all_qs:
        warn("暂无可用题目。请先生成试卷。"); wait(); return

    # Collect selection
    selected: list[str] = []  # list of "<quiz_id>:<qid>" keys

    type_label = {
        "single_choice": "单选", "multiple_choice": "多选", "true_false": "判断",
        "short_answer": "简答", "case_analysis": "案例分析",
    }

    while True:
        banner(f"🧩 组装试卷 → {subj} · {svc.lesson_name(subj, ln)}")
        show(f"已选：{Gn}{len(selected)}{R} 题")

        # Show questions grouped by quiz
        by_quiz: dict[str, list[dict]] = {}
        for q in all_qs:
            qid = q.get("_quiz_id", "unknown")
            by_quiz.setdefault(qid, []).append(q)

        # Count per quiz
        quiz_count = {qid: len(qs) for qid, qs in by_quiz.items()}
        for qid, count in quiz_count.items():
            title = by_quiz[qid][0].get("_quiz_title", qid)
            show(f"  {D}{qid}{R}: {count} 题 — {title}")

        print()
        mi(1, "按试卷浏览选题")
        mi(2, "按题号快速选择")
        mi(3, "选择全部题目")
        mi(4, "清空已选")
        mi(5, "确认并生成试卷")
        sep(); mi(0, "返回"); print()

        ch = ask("选择")
        if ch == "0":
            if selected and confirm("放弃已选题目？", default=False):
                break
            elif not selected:
                break
        elif ch == "1":
            _assemble_by_quiz(by_quiz, selected)
        elif ch == "2":
            qid_input = ask("输入题目 ID（格式：quiz_id:qid，多个逗号分隔）", allow_back=True)
            if qid_input is None:
                continue
            for part in qid_input.split(","):
                part = part.strip()
                if part:
                    # Check if it exists
                    matched = False
                    for q in all_qs:
                        key = f"{q['_quiz_id']}:{q['id']}"
                        if key == part or q['id'] == part:
                            if key not in selected:
                                selected.append(key)
                                matched = True
                            break
                    if not matched:
                        warn(f"未找到题目：{part}"); wait()
        elif ch == "3":
            for q in all_qs:
                key = f"{q['_quiz_id']}:{q['id']}"
                if key not in selected:
                    selected.append(key)
            ok(f"已选择全部 {len(all_qs)} 题"); wait()
        elif ch == "4":
            selected.clear()
            ok("已清空。"); wait()
        elif ch == "5":
            if not selected:
                warn("请至少选择一道题目。"); wait(); continue
            title = ask("试卷标题（留空=自动）", allow_back=True)
            if title is None:
                continue
            quiz = assemble_quiz(subj, ln, selected, title, svc.storage)
            if quiz is None:
                warn("组装失败，请检查题目 ID。"); wait()
            else:
                ok(f"试卷已生成：{quiz.id}（{len(quiz.questions)} 题）")
                if confirm("立即开始答题？"):
                    _run_quiz(subj, ln, quiz)
                break
    # end while


def _assemble_by_quiz(by_quiz: dict[str, list[dict]], selected: list[str]):
    """Interactive question picker grouped by source quiz."""
    while True:
        banner("🧩 按试卷选题")
        quiz_ids = list(by_quiz.keys())
        for i, qid in enumerate(quiz_ids, start=1):
            qs = by_quiz[qid]
            title = qs[0].get("_quiz_title", qid)
            show(f"  {Gn}{i}{R}. {qid} — {title}（{len(qs)} 题）")
        sep(); mi(0, "返回"); print()

        ch = ask("选择试卷", allow_back=True)
        if ch is None:
            break
        if ch == "0": break
        try:
            idx = int(ch) - 1
            if 0 <= idx < len(quiz_ids):
                _pick_questions(quiz_ids[idx], by_quiz[quiz_ids[idx]], selected)
            else:
                warn("无效编号。"); wait()
        except (ValueError, IndexError):
            warn("无效输入。"); wait()


def _pick_questions(quiz_id: str, questions: list[dict], selected: list[str]):
    """Pick individual questions from one quiz."""
    type_label = {"single_choice": "单选", "multiple_choice": "多选",
                  "true_false": "判断", "short_answer": "简答", "case_analysis": "案例分析"}

    while True:
        banner(f"🧩 选题 → {quiz_id}")
        for i, q in enumerate(questions, start=1):
            key = f"{quiz_id}:{q['id']}"
            mark = f"{Gn}✓{R}" if key in selected else " "
            tlabel = type_label.get(q.get("type", ""), q.get("type", ""))
            show(f"  {Gn}{i}{R}. {mark} [{tlabel}][{q.get('difficulty', '')}] {q.get('question', '')[:60]}")

        print()
        mi(1, "切换选中/取消")
        mi(2, "全选此试卷")
        mi(3, "取消全选此试卷")
        sep(); mi(0, "返回"); print()

        ch = ask("选择")
        if ch == "0": break
        elif ch == "1":
            idx_s = ask("题目编号（多个逗号分隔）", allow_back=True)
            if idx_s is None:
                continue
            for part in idx_s.split(","):
                try:
                    idx = int(part.strip()) - 1
                    if 0 <= idx < len(questions):
                        key = f"{quiz_id}:{questions[idx]['id']}"
                        if key in selected:
                            selected.remove(key)
                        else:
                            selected.append(key)
                except (ValueError, IndexError):
                    pass
        elif ch == "2":
            for q in questions:
                key = f"{quiz_id}:{q['id']}"
                if key not in selected:
                    selected.append(key)
        elif ch == "3":
            for q in questions:
                key = f"{quiz_id}:{q['id']}"
                if key in selected:
                    selected.remove(key)


def _run_quiz(subj: str, ln: int, quiz):
    """Run a quiz via the CLI path."""
    import subprocess, sys
    from src.storage import Storage
    from src.config_loader import get_config
    from src.exam_logic import (
        QuizAborted,
        analyze_wrong_answers_with_claude_cli,
        grade_attempt_with_claude_cli,
        mark_grading_failed,
        record_score,
        run_cli_quiz,
    )

    cfg = get_config()
    storage = Storage(cfg.course_path)

    print(f"\n📝 开始答题：{quiz.title}")
    print(f"   共 {len(quiz.questions)} 题\n")
    try:
        attempt_path, auto_score, wrong_answers = run_cli_quiz(storage, subj, quiz)
    except QuizAborted as e:
        print(f"\n{e}")
        wait()
        return

    total_possible = sum(q.score for q in quiz.questions)
    recorded_score = auto_score
    print("\n正在启动 Claude Code 试卷检测（主观题批阅、点评、薄弱点分析）...")
    agent_command = default_agent_command()
    try:
        success = grade_attempt_with_claude_cli(
            subj, ln, quiz.id, attempt_path, storage, agent_command=agent_command
        )
        if success:
            answer_record = storage.read_json(attempt_path)
            grading = answer_record.get("ai_grading", {})
            recorded_score = float(grading.get("total_score", auto_score))
            feedback_md = grading.get("feedback_md_path", "")
            ok(f"AI 试卷检测完成，总分：{recorded_score}/{total_possible}")
            if feedback_md:
                show(f"点评与薄弱点：{Path(feedback_md).resolve()}")
        else:
            mark_grading_failed(storage, attempt_path, "Claude Code CLI 未完成试卷检测")
            warn("AI 试卷检测未完成，已记录为点评失败，可稍后重试。")
    except Exception as e:
        mark_grading_failed(storage, attempt_path, e)
        warn(f"AI 试卷检测未完成：{e}")
        show("已保留原始答题记录；本次仅记录客观题自动分。")

    record_score(subj, ln, recorded_score, total_possible, storage)

    if wrong_answers:
        print(f"\n错题数：{len(wrong_answers)}")
        show("正在生成本次错题解析...")
        try:
            wrong_analysis_path = analyze_wrong_answers_with_claude_cli(
                subj,
                ln,
                wrong_answers,
                storage,
                agent_command=agent_command,
                attempt_path=attempt_path,
            )
            if wrong_analysis_path:
                show(f"错题解析：{Path(wrong_analysis_path).resolve()}")
            else:
                warn("本次错题解析未完成，可稍后在错题菜单重试。")
        except Exception as e:
            warn(f"本次错题解析未完成：{e}")
    wait()


# ═══════════════════════════════════════════════════════════════
#  Entry
# ═══════════════════════════════════════════════════════════════

def check_prereqs():
    config_path = Path(os.getenv("TUTOR_CONFIG", str(PROJECT_ROOT / "config.json")))
    if not config_path.exists():
        print(f"{Rd}未找到配置文件：{config_path}{R}"); sys.exit(1)

def main():
    check_prereqs()
    s = svc()

    if len(sys.argv) > 1:
        cmd = sys.argv[1]
        if cmd.startswith("--"):
            import subprocess
            sys.exit(subprocess.Popen([
                sys.executable, str(PROJECT_ROOT / "main.py"),
            ] + sys.argv[1:]).wait())

        # Try direct subject name
        subs = s.list_subjects()
        for sub in subs:
            if sub == cmd or sub.lower() == cmd.lower():
                menu_subject(s, sub)
                return
        print(f"未知命令: {cmd}")
        print(f"可用: 科目名 / --help")
        sys.exit(1)

    menu_main(s)

if __name__ == "__main__":
    main()
