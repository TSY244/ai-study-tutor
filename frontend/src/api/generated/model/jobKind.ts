/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export type JobKind = typeof JobKind[keyof typeof JobKind];


export const JobKind = {
  claude_connectivity_test: 'claude_connectivity_test',
  lesson_import: 'lesson_import',
  lesson_regenerate: 'lesson_regenerate',
  quiz_generate: 'quiz_generate',
  review_quiz_generate: 'review_quiz_generate',
  attempt_grade: 'attempt_grade',
  wrong_answer_analyze: 'wrong_answer_analyze',
  summary_evaluate: 'summary_evaluate',
  lesson_export: 'lesson_export',
  bilibili_cookie_test: 'bilibili_cookie_test',
} as const;
