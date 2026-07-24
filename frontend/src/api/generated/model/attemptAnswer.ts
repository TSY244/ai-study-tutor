/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { AnswerValue } from './answerValue';
import type { QuestionFeedback } from './questionFeedback';

export interface AttemptAnswer {
  question_id: string;
  answer: AnswerValue;
  /** @minimum 1 */
  answer_version: number;
  /** @nullable */
  awarded_score?: number | null;
  /** @nullable */
  is_correct?: boolean | null;
  feedback?: QuestionFeedback | null;
  updated_at: string;
}
