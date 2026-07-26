-- 回退：移除 bilibili_cookie_test job kind（要求队列中无该类型任务残留）。
ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check CHECK (kind IN (
    'claude_connectivity_test',
    'lesson_import',
    'lesson_regenerate',
    'quiz_generate',
    'review_quiz_generate',
    'attempt_grade',
    'wrong_answer_analyze',
    'summary_evaluate',
    'lesson_export'
));
