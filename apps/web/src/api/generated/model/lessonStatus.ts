/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export type LessonStatus = typeof LessonStatus[keyof typeof LessonStatus];


export const LessonStatus = {
  draft: 'draft',
  processing: 'processing',
  ready: 'ready',
  failed: 'failed',
  ready_with_warning: 'ready_with_warning',
} as const;
