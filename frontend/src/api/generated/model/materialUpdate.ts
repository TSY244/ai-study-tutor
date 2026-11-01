/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface MaterialUpdate {
  /** @minimum 1 */
  version: number;
  /** @maxLength 300 */
  title?: string;
  /**
     * @minLength 1
     * @maxLength 500000
     */
  text?: string;
  source_url?: string;
  /** @maxLength 30 */
  preferred_language?: string;
}
