/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export type SummaryEvaluationStatus = typeof SummaryEvaluationStatus[keyof typeof SummaryEvaluationStatus];


export const SummaryEvaluationStatus = {
  queued: 'queued',
  evaluating: 'evaluating',
  succeeded: 'succeeded',
  failed: 'failed',
} as const;
