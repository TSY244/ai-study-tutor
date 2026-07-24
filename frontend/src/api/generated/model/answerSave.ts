/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { AnswerValue } from './answerValue';

export interface AnswerSave {
  answer: AnswerValue;
  /** @minimum 0 */
  answer_version: number;
  client_updated_at: string;
}
