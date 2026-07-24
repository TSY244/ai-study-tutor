/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { JobEventData } from './jobEventData';
import type { JobEventEventType } from './jobEventEventType';

export interface JobEvent {
  /** @minimum 1 */
  id: number;
  job_id: string;
  event_type: JobEventEventType;
  stage: string;
  /**
     * @minimum 0
     * @maximum 100
     */
  progress: number;
  /** @nullable */
  message: string | null;
  data?: JobEventData;
  occurred_at: string;
}
