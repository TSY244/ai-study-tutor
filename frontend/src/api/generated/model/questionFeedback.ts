/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { AnswerValue } from './answerValue';

export interface QuestionFeedback {
  question_id: string;
  /** @minimum 0 */
  awarded_score: number;
  /** @exclusiveMinimum 0 */
  max_score: number;
  /** @nullable */
  is_correct: boolean | null;
  feedback: string;
  correct_understanding: string;
  correct_answer: AnswerValue | null;
  /** @nullable */
  explanation: string | null;
  weak_concepts: string[];
}
