/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface Course {
  id: string;
  title: string;
  description: string;
  archived: boolean;
  /** @minimum 0 */
  lesson_count: number;
  version: number;
  created_at: string;
  updated_at: string;
}
