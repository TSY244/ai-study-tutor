/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { JobError } from './jobError';
import type { JobGenerationFamily } from './jobGenerationFamily';
import type { JobKind } from './jobKind';
import type { JobStatus } from './jobStatus';

export interface Job {
  id: string;
  kind: JobKind;
  /** @nullable */
  aggregate_type?: string | null;
  /** @nullable */
  aggregate_id?: string | null;
  /** @nullable */
  generation_family?: JobGenerationFamily;
  /** @nullable */
  retry_of_job_id?: string | null;
  status: JobStatus;
  stage: string;
  /**
     * @minimum 0
     * @maximum 100
     */
  progress: number;
  /** @nullable */
  message?: string | null;
  /** @minimum 0 */
  attempt_count: number;
  /** @minimum 1 */
  max_attempts: number;
  retryable: boolean;
  cancel_requested: boolean;
  error?: JobError | null;
  created_at: string;
  /** @nullable */
  started_at?: string | null;
  /** @nullable */
  completed_at?: string | null;
  updated_at: string;
}
