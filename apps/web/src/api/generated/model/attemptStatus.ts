/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export type AttemptStatus = typeof AttemptStatus[keyof typeof AttemptStatus];


export const AttemptStatus = {
  in_progress: 'in_progress',
  submitted: 'submitted',
  grading: 'grading',
  graded: 'graded',
  grading_failed: 'grading_failed',
} as const;
