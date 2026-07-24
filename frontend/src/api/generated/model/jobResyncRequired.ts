/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface JobResyncRequired {
  reason: 'history_expired';
  /** @minimum 0 */
  high_watermark_event_id: number;
  active_jobs_url: '/api/v1/jobs?active=true';
}
