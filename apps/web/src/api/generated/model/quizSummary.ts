/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { QuizSummarySource } from './quizSummarySource';

export interface QuizSummary {
  id: string;
  lesson_id: string;
  title: string;
  source: QuizSummarySource;
  /** @minimum 0 */
  question_count: number;
  created_at: string;
}
