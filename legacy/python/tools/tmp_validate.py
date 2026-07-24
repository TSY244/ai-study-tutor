import json
from collections import Counter

with open('/Users/august/code/vsc/python/learning/ai-study-tutor/course/基金/lesson_03/exam/quiz_chapter_03_51e6a4.json') as f:
    data = json.load(f)

errors = []

if data.get('id') != 'quiz_chapter_03_51e6a4':
    errors.append('id mismatch')
if data.get('course_id') != '基金':
    errors.append('course_id mismatch')
if data.get('chapter_id') != 3:
    errors.append('chapter_id mismatch')

qs = data.get('questions', [])
if len(qs) != 11:
    errors.append(f'Expected 11 questions, got {len(qs)}')

diffs = Counter(q['difficulty'] for q in qs)
for d, e in [('easy', 2), ('medium', 6), ('hard', 2), ('challenge', 1)]:
    if diffs.get(d, 0) != e:
        errors.append(f'{d} count: {diffs.get(d, 0)} (expected {e})')

diff_order = {'easy': 0, 'medium': 1, 'hard': 2, 'challenge': 3}
prev = -1
for i, q in enumerate(qs):
    cur = diff_order[q['difficulty']]
    if cur < prev:
        errors.append(f'Difficulty not increasing at q{i+1}')
        break
    prev = cur

cogs = Counter(q['cognitive_level'] for q in qs)
for c, e in [('recall', 6), ('explain', 3), ('apply', 2)]:
    if cogs.get(c, 0) != e:
        errors.append(f'{c} count: {cogs.get(c, 0)} (expected {e})')

types = set(q['type'] for q in qs)
for t in ['single_choice', 'multiple_choice', 'true_false', 'short_answer', 'case_analysis']:
    if t not in types:
        errors.append(f'Missing question type: {t}')

for q in qs:
    for field in ['tested_concepts', 'difficulty', 'cognitive_level', 'score', 'explanation']:
        if field not in q:
            errors.append(f'q{q["id"]} missing {field}')

total = sum(q['score'] for q in qs)
if total != 100:
    errors.append(f'Total score: {total}, expected 100')

all_text = json.dumps(data, ensure_ascii=False)
co = all_text.count('"')
co = all_text.count('“')
cc = all_text.count('”')
print(f'Curly open quotes (U+201C): {co}')
print(f'Curly close quotes (U+201D): {cc}')

weak_points = ['分批止盈', '加仓触发', '市场环境', '资金分配', '执行纪律', '投资心理', '前提条件']
weak_q_count = sum(
    1 for q in qs
    if any(wp in ' '.join(q.get('tested_concepts', [])) for wp in weak_points)
)
print(f'Weak point questions: {weak_q_count}/11 ({100*weak_q_count//11}%)')
if weak_q_count < 4:
    errors.append(f'Weak points: {weak_q_count} (< 4)')

if errors:
    print(f'ERRORS ({len(errors)}):')
    for e in errors:
        print(f'  - {e}')
else:
    print('ALL VALIDATIONS PASSED')

print(f'Difficulty: {dict(diffs)}')
print(f'Cognitive: {dict(cogs)}')
print(f'Types: {sorted(types)}')
print(f'Total score: {total}')
for i, q in enumerate(qs):
    print(f'  q{i+1:2d} | {q["id"]:3s} | {q["type"]:15s} | {q["difficulty"]:10s} | {q["cognitive_level"]:8s} | {q["score"]:2d}')
