/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface QuizGenerationCreate {
  /**
     * @minimum 5
     * @maximum 15
     * @nullable
     */
  question_count?: number | null;
  /**
     * @maxLength 300
     * @nullable
     */
  title?: string | null;
}
