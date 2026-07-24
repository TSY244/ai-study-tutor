/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface ProgressPoint {
  recorded_at: string;
  /**
     * @minimum 0
     * @maximum 1
     */
  mastery: number;
  formula_version: string;
  /** @nullable */
  source_attempt_id: string | null;
}
