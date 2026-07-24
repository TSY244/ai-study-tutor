/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface CourseUpdate {
  /**
     * @minLength 1
     * @maxLength 200
     */
  title?: string;
  /** @maxLength 5000 */
  description?: string;
  archived?: boolean;
  /** @minimum 1 */
  version: number;
}
