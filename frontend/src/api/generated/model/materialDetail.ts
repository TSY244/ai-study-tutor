/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { MaterialDetailStatus } from './materialDetailStatus';
import type { MaterialKind } from './materialKind';

export interface MaterialDetail {
  /** @minimum 1 */
  version?: number;
  id: string;
  lesson_id: string;
  kind: MaterialKind;
  status: MaterialDetailStatus;
  /** @nullable */
  source_url?: string | null;
  /** @nullable */
  original_name?: string | null;
  created_at: string;
  /** @nullable */
  text?: string | null;
  /** @nullable */
  title?: string | null;
  /** @nullable */
  preferred_language?: string | null;
}
