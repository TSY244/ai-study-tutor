/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { AnswerValue } from './answerValue';

export interface SavedAnswer {
  attempt_id: string;
  question_id: string;
  answer: AnswerValue;
  /** @minimum 1 */
  answer_version: number;
  /** @minimum 1 */
  attempt_version: number;
  saved_at: string;
}
