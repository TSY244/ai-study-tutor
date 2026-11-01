/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export type ClaudeKeyStatus = typeof ClaudeKeyStatus[keyof typeof ClaudeKeyStatus];


export const ClaudeKeyStatus = {
  unknown: 'unknown',
  healthy: 'healthy',
  cooling_down: 'cooling_down',
  auth_failed: 'auth_failed',
  quota_exhausted: 'quota_exhausted',
  disabled: 'disabled',
  testing: 'testing',
} as const;
