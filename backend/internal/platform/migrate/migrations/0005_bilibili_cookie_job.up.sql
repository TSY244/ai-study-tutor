-- bilibili_cookie_test：B 站 Cookie 可用性检测任务（Worker 执行，API 无 egress）。
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
    'lesson_export',
    'bilibili_cookie_test'
));
