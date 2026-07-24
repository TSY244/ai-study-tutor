/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { JobStatus } from './jobStatus';

export interface JobStreamData {
  job_id: string;
  status: JobStatus;
  stage: string;
  /**
     * @minimum 0
     * @maximum 100
     */
  progress: number;
  /** @nullable */
  message: string | null;
  occurred_at: string;
}
