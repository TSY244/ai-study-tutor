/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { MaterialKind } from './materialKind';
import type { MaterialStatus } from './materialStatus';

export interface Material {
  /** @minimum 1 */
  version?: number;
  id: string;
  lesson_id: string;
  kind: MaterialKind;
  status: MaterialStatus;
  /** @nullable */
  source_url?: string | null;
  /** @nullable */
  original_name?: string | null;
  created_at: string;
}
