/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export type JobStatus = typeof JobStatus[keyof typeof JobStatus];


export const JobStatus = {
  queued: 'queued',
  running: 'running',
  validating: 'validating',
  succeeded: 'succeeded',
  failed: 'failed',
  cancel_requested: 'cancel_requested',
  cancelled: 'cancelled',
} as const;
