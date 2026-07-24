/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface TextMaterialCreate {
  /** @maxLength 200 */
  title?: string;
  /**
     * @minLength 1
     * @maxLength 1000000
     */
  text: string;
}
