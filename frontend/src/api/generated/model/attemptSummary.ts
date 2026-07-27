/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { AttemptStatus } from './attemptStatus';

export interface AttemptSummary {
  id: string;
  quiz_id: string;
  quiz_title: string;
  status: AttemptStatus;
  /** @nullable */
  auto_score?: number | null;
  /** @nullable */
  final_score?: number | null;
  /** @nullable */
  max_score?: number | null;
  started_at: string;
  /** @nullable */
  submitted_at?: string | null;
  /** @nullable */
  graded_at?: string | null;
}
