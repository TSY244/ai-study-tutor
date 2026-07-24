/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { DocumentKind } from './documentKind';

export interface Document {
  id: string;
  lesson_id: string;
  kind: DocumentKind;
  /** @minimum 1 */
  revision: number;
  content_md: string;
  /** @nullable */
  source_job_id: string | null;
  is_active: boolean;
  created_at: string;
}
