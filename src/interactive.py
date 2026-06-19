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

from src.course_service import CourseService, svc

# ── Colors ──
B = "\033[1m"; D = "\033[2m"; R = "\033[0m"
Rd = "\033[31m"; Gn = "\033[32m"; Yw = "\033[33m"; Cy = "\033[36m"

# ── Helpers ──

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

def ask(prompt: str, default: str = "") -> str:
    d = f" {D}[{default}]{R}" if default else ""
    try:
        return input(f"  {Gn}?{R} {prompt}{d}: ").strip() or default
    except (EOFError, KeyboardInterrupt):
        print(); return ""

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
    print()
    ch = ask("输入编号或名称", "1")
    if not ch:
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
    print()
    ch = ask("课时编号", str(lessons[-1]))
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

        mi(1, "生成新试卷",    f"{Gn}已生成{R}" if status["quiz"] else f"{D}未生成{R}")
        mi(2, "开始答题",      "")
        mi(3, "历史试卷",      "")
        mi(4, "组装试卷",      "人工从历史试卷选题")
        mi(5, "错题复习考试",  "")
        mi(6, "查看/解析错题", f"{Rd}{wc} 道{R}" if wc else f"{D}无{R}")
        mi(7, "查看成绩",      "")
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

# ═══════════════════════════════════════════════════════════════
#  Actions (call service layer)
# ═══════════════════════════════════════════════════════════════

def _create_subject(svc: CourseService):
    banner("📁 新建科目")
    subs = svc.list_subjects()
    if subs:
        show("已有科目：" + "、".join(subs))
        print()
    name = ask("新科目名称（如：高等数学）")
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
    ch = ask("科目编号")
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
    ln_str = ask("课时编号", default_ln)
    ln = int(ln_str) if ln_str.isdigit() else 1
    name = ask("课时名称", f"第{ln}课")

    print(f"\n  {B}素材来源：{R}")
    print(f"    {Gn}1{R}  视频链接（YouTube / B站）← Claude Code 自动获取")
    print(f"    {Gn}2{R}  本地文件")
    print(f"    {Gn}3{R}  直接粘贴文本")
    st = ask("选择", "1")

    flag, value = "", ""
    if st == "1":
        flag, value = "--load-url", ask("视频链接")
    elif st == "2":
        flag, value = "--load-video", ask("文件路径")
    elif st == "3":
        flag, value = "--source-text", ""
        print(f"  {Yw}输入文本（单独一行 END 结束）：{R}")
        lines = []
        while True:
            try:
                line = input()
            except (EOFError, KeyboardInterrupt):
                print(); break
            if line.strip().upper() == "END":
                break
            lines.append(line)
        value = "\n".join(lines)
        if not value.strip():
            err("内容不能为空。"); wait(); return
        hint(f"已输入 {len(value)} 字符")
    else:
        err("无效选择。"); wait(); return

    model = ask("AI 模型", "deepseek-v4-pro")

    # Confirm
    print(f"\n  {B}{'─' * 44}{R}")
    show(f"科目：{Gn}{subj}{R}")
    show(f"课时：{Gn}第{ln}课 · {name}{R}")
    show(f"素材：{Gn}{value[:70]}{'...' if len(value) > 70 else ''}{R}")
    show(f"模型：{Gn}{model}{R}")
    sep()
    if not confirm("确认排队导入任务？"):
        show("已取消。"); wait(); return

    task_id = svc.queue_import_lesson(subj, ln, name, flag, value, model)
    ok(f"任务已排队（{task_id}）")

    from src.agent_runner import process_all_pending_via_claude
    succeeded, failed = process_all_pending_via_claude()

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
        print(svc.storage.read_text(student_path))
    else:
        warn("学生笔记尚未生成。请先导入课时。")
    wait()

def _view_teacher_notes(svc: CourseService, subj: str, ln: int):
    """教师视图：只看教师教纲。"""
    banner(f"📖 教师教纲 → {subj} · {svc.lesson_name(subj, ln)}")
    teacher_path = svc.storage.teacher_notes_path(subj, ln)
    if teacher_path.exists():
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
    ch = ask("课时编号")
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
    model = ask("AI 模型", "deepseek-v4-pro")
    print(f"\n  {Yw}输入你的学习总结（单独一行 END 结束）：{R}")
    lines = []
    while True:
        try:
            line = input()
        except (EOFError, KeyboardInterrupt):
            print(); break
        if line.strip().upper() == "END":
            break
        lines.append(line)
    text = "\n".join(lines)
    if not text.strip():
        err("内容不能为空。"); wait(); return

    from .agent_runner import queue_summarize_task

    # Gather course content
    course_content = ""
    student_path = svc.storage.student_notes_path(subj, ln)
    if student_path.exists():
        course_content = svc.storage.read_text(student_path)

    output_dir = svc.storage.lesson_dir(subj, ln)
    task_id = queue_summarize_task(subj, ln, text, course_content, output_dir, model)
    ok(f"评估任务已排队（{task_id}）")

    from src.agent_runner import process_all_pending_via_claude
    succeeded, failed = process_all_pending_via_claude()

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
    ch = ask("选择", "1")
    if ch == "1":
        import subprocess, sys
        subprocess.Popen([
            sys.executable, str(PROJECT_ROOT / "main.py"),
            "--export-md", "--subject", subj, "--lesson", str(ln),
        ]).wait()
    elif ch == "2":
        model = ask("AI 模型", "deepseek-v4-pro")
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
            source_text, output_dir, model,
        )
        ok(f"重新生成任务已排队（{task_id}）")

        from src.agent_runner import process_all_pending_via_claude
        succeeded, failed = process_all_pending_via_claude()

        if succeeded > 0:
            ok(f"处理完毕！成功 {succeeded} 个任务。按 {Gn}{ln}VS{R} 查看笔记。")
        else:
            warn("自动处理未完成，任务已保存。")
            show(f"{Cy}请在 Claude Code 中说「处理所有待处理的学习任务」{R}")
    wait()

def _exam_new(svc: CourseService, subj: str, ln: int):
    """Queue exam generation task — Claude Code CLI processes it automatically."""
    from src.agent_runner import process_task_via_claude_cli

    num = ask("题目数量（留空=策略自动）", "")
    num_int = int(num) if num.strip() else None
    model = ask("AI 模型", "deepseek-v4-pro")

    task_id = svc.queue_exam_generation(subj, ln, num_int, model)

    # Automatically invoke Claude Code CLI to process the task
    success = process_task_via_claude_cli(task_id)

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

    model = ask("AI 模型", "deepseek-v4-pro")

    task_id = svc.queue_exam_generation(subj, ln, None, model)

    # Automatically invoke Claude Code CLI to process the task
    success = process_task_via_claude_cli(task_id)

    if success:
        ok("复习试卷生成完毕！")
    else:
        warn("自动生成未完成，任务已保存。")
        show(f"{Cy}请在 Claude Code 中说「处理所有待处理的学习任务」{R}")

    wait()

def _exam_wrong(svc: CourseService, subj: str, ln: int):
    wc = svc.wrong_count(subj, ln)
    show(f"当前 {Rd}{wc} 道错题{R}" if wc else f"{D}暂无错题 🎉{R}")
    print(f"\n  {Gn}1{R}  查看错题列表")
    print(f"  {Gn}2{R}  AI 生成错题解析")
    ch = ask("选择", "1")
    import subprocess, sys
    if ch == "1":
        subprocess.Popen([
            sys.executable, str(PROJECT_ROOT / "main.py"),
            "--wrong-list", "--subject", subj, "--lesson", str(ln),
        ]).wait()
    elif ch == "2":
        model = ask("AI 模型", "deepseek-v4-pro")
        subprocess.Popen([
            sys.executable, str(PROJECT_ROOT / "main.py"),
            "--fix-wrong", "--subject", subj, "--lesson", str(ln), "--model", model,
        ]).wait()
    wait()

def _exam_score(svc: CourseService, subj: str, ln: int):
    import subprocess, sys
    subprocess.Popen([
        sys.executable, str(PROJECT_ROOT / "main.py"),
        "--score", "--subject", subj, "--lesson", str(ln),
    ]).wait()
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
            idx_s = ask("试卷编号", "1")
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
            idx_s = ask("试卷编号", "1")
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
            qid_input = ask("输入题目 ID（格式：quiz_id:qid，多个逗号分隔）")
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
            title = ask("试卷标题（留空=自动）")
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

        ch = ask("选择试卷")
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
            idx_s = ask("题目编号（多个逗号分隔）")
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
    from src.exam_logic import run_cli_quiz, record_score

    cfg = get_config()
    storage = Storage(cfg.course_path)

    print(f"\n📝 开始答题：{quiz.title}")
    print(f"   共 {len(quiz.questions)} 题\n")
    attempt_path, auto_score, wrong_answers = run_cli_quiz(storage, subj, quiz)

    total_possible = sum(q.score for q in quiz.questions)
    record_score(subj, ln, auto_score, total_possible, storage)

    if wrong_answers:
        print(f"\n错题数：{len(wrong_answers)}")
    wait()


# ═══════════════════════════════════════════════════════════════
#  Entry
# ═══════════════════════════════════════════════════════════════

def check_prereqs():
    if not (PROJECT_ROOT / "config.json").exists():
        print(f"{Rd}未找到 config.json{R}"); sys.exit(1)

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
