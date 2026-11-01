/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { Question } from './question';
import type { QuizSource } from './quizSource';
import type { QuizStatus } from './quizStatus';

export interface Quiz {
  /** @minimum 1 */
  version?: number;
  /** 本次生成使用的出题备注；旧试卷为空字符串。 */
  generation_notes?: string;
  id: string;
  lesson_id: string;
  title: string;
  source: QuizSource;
  status: QuizStatus;
  questions: Question[];
  created_at: string;
}
