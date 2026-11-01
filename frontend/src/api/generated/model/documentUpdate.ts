/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface DocumentUpdate {
  /**
     * @minLength 1
     * @maxLength 200000
     */
  content_md: string;
  /** @minimum 1 */
  expected_revision: number;
}
