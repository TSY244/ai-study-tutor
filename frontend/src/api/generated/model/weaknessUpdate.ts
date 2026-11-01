/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface WeaknessUpdate {
  /** @minimum 1 */
  version: number;
  /**
     * @minLength 1
     * @maxLength 300
     */
  concept_key?: string;
  /**
     * @minLength 1
     * @maxLength 300
     */
  concept_label?: string;
  /** @maxLength 10000 */
  description?: string;
  /**
     * @minimum 1
     * @maximum 5
     */
  severity?: number;
  mastered?: boolean;
}
