/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface ReviewQuizGenerationCreate {
  /**
     * @minimum 5
     * @maximum 15
     * @nullable
     */
  question_count?: number | null;
  include_lesson_ids?: string[];
  weakness_ids?: string[];
}
