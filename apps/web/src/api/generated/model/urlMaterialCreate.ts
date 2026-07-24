/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface UrlMaterialCreate {
  /** @pattern ^https:// */
  url: string;
  /** @maxLength 30 */
  preferred_language?: string;
}
