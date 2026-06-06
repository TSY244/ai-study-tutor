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
        mi(3, "错题复习考试",  "")
        mi(4, "查看/解析错题", f"{Rd}{wc} 道{R}" if wc else f"{D}无{R}")
        mi(5, "查看成绩",      "")
        sep(); mi(0, "返回科目"); print()

        ch = ask("选择")
        if ch == "0": break
        elif ch == "1": _exam_new(svc, subj, ln)
        elif ch == "2":
            if not status["quiz"]: warn("请先生成试卷。"); wait(); continue
            _exam_take(svc, subj, ln)
        elif ch == "3": _exam_review(svc, subj, ln)
        elif ch == "4": _exam_wrong(svc, subj, ln)
        elif ch == "5": _exam_score(svc, subj, ln)

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
    show(f"{Cy}请切换到 Claude Code 并说「处理所有待处理的学习任务」{R}")
    show(f"处理完成后返回此菜单，按 {Gn}{ln}VS{R} 查看学生笔记。")
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
        mi(2, "错题复习考试",    f"{Rd}{wc} 道错题{R}" if wc else "")
        mi(3, "查看/解析错题",   f"{Rd}{wc} 道{R}" if wc else f"{D}无{R}")
        mi(4, "查看成绩",        "")
        sep(); mi(0, "返回科目"); print()

        ch = ask("选择")
        if ch == "0": break
        elif ch == "1": _exam_new(svc, subj, ln)
        elif ch == "2":
            if wc == 0: warn("暂无错题。"); wait(); continue
            _exam_review(svc, subj, ln)
        elif ch == "3": _exam_wrong(svc, subj, ln)
        elif ch == "4": _exam_score(svc, subj, ln)

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
    show(f"{Cy}请切换到 Claude Code 并说「处理所有待处理的学习任务」{R}")
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
        show(f"{Cy}请切换到 Claude Code 并说「处理所有待处理的学习任务」{R}")
    wait()

def _exam_new(svc: CourseService, subj: str, ln: int):
    num = ask("题目数量", "10")
    model = ask("AI 模型", "deepseek-v4-pro")
    task_id = svc.queue_exam_generation(subj, ln, int(num), model)
    ok(f"出题任务已排队（{task_id}）")
    show(f"{Cy}请切换到 Claude Code 并说「处理所有待处理的学习任务」{R}")
    wait()

def _exam_take(svc: CourseService, subj: str, ln: int):
    import subprocess, sys
    subprocess.Popen([
        sys.executable, str(PROJECT_ROOT / "main.py"),
        "--exam", "--subject", subj, "--lesson", str(ln),
    ]).wait()
    wait()

def _exam_review(svc: CourseService, subj: str, ln: int):
    model = ask("AI 模型", "deepseek-v4-pro")
    task_id = svc.queue_exam_generation(subj, ln, 8, model)
    ok(f"复习出题任务已排队（{task_id}）")
    show(f"{Cy}请切换到 Claude Code 并说「处理所有待处理的学习任务」{R}")
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
