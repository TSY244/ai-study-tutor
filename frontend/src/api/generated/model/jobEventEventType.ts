/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export type JobEventEventType = typeof JobEventEventType[keyof typeof JobEventEventType];


export const JobEventEventType = {
  jobqueued: 'job.queued',
  jobstarted: 'job.started',
  jobprogress: 'job.progress',
  jobvalidating: 'job.validating',
  jobcancel_requested: 'job.cancel_requested',
  jobsucceeded: 'job.succeeded',
  jobfailed: 'job.failed',
  jobcancelled: 'job.cancelled',
} as const;
