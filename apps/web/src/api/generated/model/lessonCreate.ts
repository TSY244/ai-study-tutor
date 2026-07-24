/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface LessonCreate {
  /** @minimum 1 */
  ordinal: number;
  /**
     * @minLength 1
     * @maxLength 200
     */
  title: string;
  /** @maxLength 5000 */
  description?: string;
}
