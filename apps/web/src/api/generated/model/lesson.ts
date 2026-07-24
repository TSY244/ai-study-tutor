/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { DocumentAvailability } from './documentAvailability';
import type { LessonStatus } from './lessonStatus';

export interface Lesson {
  id: string;
  course_id: string;
  ordinal: number;
  title: string;
  description: string;
  status: LessonStatus;
  /** @nullable */
  active_job_id?: string | null;
  documents: DocumentAvailability;
  /**
     * @minimum 0
     * @maximum 1
     * @nullable
     */
  mastery?: number | null;
  version: number;
  created_at: string;
  updated_at: string;
}
