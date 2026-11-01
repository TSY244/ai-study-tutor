/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface QuizGenerationCreate {
  /**
     * 本次出题重点、题型比例、难度及覆盖范围偏好。
     * @maxLength 3000
     */
  generation_notes?: string;
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
