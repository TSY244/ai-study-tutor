/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { QuestionInput } from './questionInput';

export interface ManualQuizUpdate {
  /** @minimum 1 */
  version: number;
  /**
     * @minLength 1
     * @maxLength 300
     */
  title?: string;
  /** @maxLength 3000 */
  generation_notes?: string;
  /**
     * @minItems 1
     * @maxItems 100
     */
  questions?: QuestionInput[];
}
