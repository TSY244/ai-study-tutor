"""Prompt templates for AI interactions (Claude format)."""

from __future__ import annotations

import json
from typing import Any

JSON_SYSTEM = (
    "你必须遵守：\n"
    "1. 只能基于用户提供的学习资料生成内容，资料没有提及的内容不要编造。\n"
    "2. 输出必须是严格 JSON，不要 Markdown，不要代码块，不要额外解释。\n"
    "3. 如果资料不足，请在 JSON 字段中用简短文字说明不足，而不是补充虚构事实。\n"
    "4. 中文表达要清晰、适合初学者。"
)


def trim_text(text: str, max_chars: int = 24000) -> str:
    if len(text) <= max_chars:
        return text
    head = text[: max_chars // 2]
    tail = text[-max_chars // 2 :]
    return f"{head}\n\n...[中间内容过长已截断]...\n\n{tail}"


def outline_prompt(raw_text: str) -> tuple[str, str]:
    """Return (system, user_prompt) for generating a course outline."""
    schema = {
        "course_title": "课程标题",
        "target_audience": "目标学习者",
        "chapters": [
            {
                "chapter_id": 1,
                "title": "章节标题",
                "learning_goals": ["学习目标"],
                "key_concepts": ["核心概念"],
                "difficulty": "beginner/intermediate/advanced",
                "estimated_minutes": 30,
            }
        ],
    }
    user = (
        "请根据以下学习资料生成课程大纲。章节数量以资料结构为准，MVP 建议 4-10 章。\n"
        f"JSON Schema 示例：\n{json.dumps(schema, ensure_ascii=False, indent=2)}\n\n"
        f"学习资料：\n{trim_text(raw_text)}"
    )
    return JSON_SYSTEM, user


def lesson_prompt(
    chapter_id: int,
    chapter_title: str,
    chapter_content: str,
    learning_goals: list[str],
    key_concepts: list[str],
) -> tuple[str, str]:
    """Return (system, user_prompt) for generating a chapter lesson."""
    system = (
        "你是一位中文学习助教。只能基于用户提供的资料讲解，不要编造资料外事实。"
        "输出 Markdown，不要输出 JSON。"
    )
    user = (
        f"请为第 {chapter_id} 章《{chapter_title}》生成适合初学者的教学文案。\n"
        f"学习目标：{learning_goals}\n"
        f"关键概念：{key_concepts}\n\n"
        "Markdown 结构必须包含：\n"
        f"# 第 {chapter_id} 章：{chapter_title}\n"
        "## 本章目标\n## 为什么要学这一章\n## 先修知识\n## 核心概念\n"
        "## 详细讲解\n## 案例\n## 常见误区\n## 本章小结\n## 自测清单\n## 复习建议\n\n"
        f"章节资料：\n{trim_text(chapter_content)}"
    )
    return system, user


def quiz_prompt(
    chapter_content: str,
    lesson_markdown: str,
    weaknesses: list[dict[str, Any]],
    question_count: int = 10,
) -> tuple[str, str]:
    """Return (system, user_prompt) for generating a quiz."""
    schema = {
        "quiz_title": "章节测试标题",
        "questions": [
            {
                "id": "q1",
                "type": "single_choice",
                "question": "题干",
                "options": {"A": "选项A", "B": "选项B", "C": "选项C", "D": "选项D"},
                "correct_answer": "A",
                "explanation": "解析",
                "tested_concepts": ["概念"],
                "difficulty": "easy",
                "score": 5,
            }
        ],
    }
    user = (
        f"请生成 {question_count} 道章节考试题。题型需覆盖 single_choice、multiple_choice、"
        "true_false、short_answer、case_analysis。每题必须有 tested_concepts 和 score。\n"
        f"JSON Schema 示例：\n{json.dumps(schema, ensure_ascii=False, indent=2)}\n\n"
        f"历史薄弱点：\n{json.dumps(weaknesses, ensure_ascii=False, indent=2)}\n\n"
        f"章节资料：\n{trim_text(chapter_content, 16000)}\n\n"
        f"教学文案：\n{trim_text(lesson_markdown, 12000)}"
    )
    return JSON_SYSTEM, user


def grade_prompt(quiz: dict[str, Any], answers: list[dict[str, str]]) -> tuple[str, str]:
    """Return (system, user_prompt) for grading."""
    schema = {
        "total_score": 85,
        "question_feedback": [
            {
                "question_id": "q1",
                "score": 5,
                "is_correct": True,
                "feedback": "批阅意见",
                "correct_understanding": "正确理解",
                "weak_concepts": [],
            }
        ],
        "weak_concepts": [
            {
                "concept": "概念",
                "weakness_description": "薄弱描述",
                "severity": 4,
                "evidence": "证据",
            }
        ],
        "review_plan": ["复习建议"],
        "retest_recommended": True,
    }
    user = (
        "请批阅用户答案。客观题按标准答案判分，主观题按理解完整度给分。"
        "必须指出错因、正确理解、薄弱知识点和复习建议。\n"
        f"输出 JSON Schema 示例：\n{json.dumps(schema, ensure_ascii=False, indent=2)}\n\n"
        f"试题与标准答案：\n{json.dumps(quiz, ensure_ascii=False, indent=2)}\n\n"
        f"用户答案：\n{json.dumps(answers, ensure_ascii=False, indent=2)}"
    )
    return JSON_SYSTEM, user


def retest_prompt(
    chapter_content: str,
    weaknesses: list[dict[str, Any]],
    previous_quiz: dict[str, Any] | None,
    previous_feedback: dict[str, Any] | None,
    question_count: int = 8,
) -> tuple[str, str]:
    """Return (system, user_prompt) for generating a retest."""
    user = (
        f"请生成 {question_count} 道二次考试题。要求：不重复原题，换角度考察同一概念，"
        "重点围绕历史薄弱点，增加案例题和解释题。\n"
        "输出格式与普通试题生成相同：包含 quiz_title 和 questions。\n\n"
        f"章节资料：\n{trim_text(chapter_content, 16000)}\n\n"
        f"历史薄弱点：\n{json.dumps(weaknesses, ensure_ascii=False, indent=2)}\n\n"
        f"上次试题：\n{json.dumps(previous_quiz or {}, ensure_ascii=False, indent=2)}\n\n"
        f"上次批阅反馈：\n{json.dumps(previous_feedback or {}, ensure_ascii=False, indent=2)}"
    )
    return JSON_SYSTEM, user


# --- New prompts for teacher/student notes and summary check ---

def teacher_notes_prompt(raw_text: str, subject: str, lesson_name: str) -> tuple[str, str]:
    """Generate teacher outline/syllabus."""
    system = (
        "你是一位资深教师。请根据提供的教学内容生成一份详细的教师教纲（Markdown格式）。"
        "教纲应该结构化、清晰，适合教师授课时参考。只基于资料内容，不要编造。"
    )
    user = (
        f"请根据以下教学内容，生成一份教师教纲（Markdown）。\n"
        f"科目：{subject}\n"
        f"课时名称：{lesson_name}\n\n"
        "教纲结构应包含：\n"
        "# 教师教纲\n"
        "## 教学目标\n## 教学重难点\n## 教学流程（导入→讲解→练习→总结）\n"
        "## 板书设计\n## 课后作业建议\n## 教学反思要点\n\n"
        f"教学内容：\n{trim_text(raw_text)}"
    )
    return system, user


def student_notes_prompt(teacher_notes: str) -> tuple[str, str]:
    """Generate detailed student notes from the teacher outline."""
    system = (
        "你是一位善于将复杂知识简单化的学习助教。请根据教纲生成一份适合学生自学的详细笔记。"
        "要求：通俗易懂，举例丰富，重点标注清晰，只基于教纲内容。"
    )
    user = (
        "根据下述教纲生成详细的学生学习笔记（Markdown）。\n"
        "要求：补充具体举例，标注重难点，加入记忆技巧和常见误区提醒。\n\n"
        f"教师教纲：\n{trim_text(teacher_notes)}"
    )
    return system, user


def summary_check_prompt(raw_text: str, student_summary: str, subject: str) -> tuple[str, str]:
    """Evaluate a student's self-summary against source material."""
    system = JSON_SYSTEM
    schema = {
        "accuracy_score": 85,
        "completeness_score": 70,
        "overall_assessment": "总体评价",
        "key_misunderstandings": ["理解偏差点"],
        "missing_concepts": ["遗漏的重要概念"],
        "strengths": ["理解较好的部分"],
        "suggestions": ["改进建议"],
    }
    user = (
        f"请评估学生对以下学习内容的自我总结。比对原始资料，评估理解准确度和完整度。\n\n"
        f"科目：{subject}\n\n"
        f"原始资料：\n{trim_text(raw_text, 12000)}\n\n"
        f"学生总结：\n{student_summary}\n\n"
        f"请按以下 JSON schema 输出：\n"
        f"{json.dumps(schema, ensure_ascii=False, indent=2)}"
    )
    return system, user


def fix_wrong_prompt(wrong_question: dict[str, Any], course_content: str) -> tuple[str, str]:
    """Generate detailed analysis for a wrong question."""
    system = (
        "你是学习分析专家。请针对学生的错题，给出详细解析："
        "分析错因、讲解正确思路、提供类似题目供练习。输出 Markdown。"
    )
    user = (
        "请分析以下错题，给出：\n"
        "1. 错因分析\n"
        "2. 正确解题思路\n"
        "3. 相关知识点讲解\n"
        "4. 一道类似练习题\n\n"
        f"课程内容参考：\n{trim_text(course_content, 8000)}\n\n"
        f"错题信息：\n{json.dumps(wrong_question, ensure_ascii=False, indent=2)}"
    )
    return system, user
