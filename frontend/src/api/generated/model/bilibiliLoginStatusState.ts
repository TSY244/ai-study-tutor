/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export type BilibiliLoginStatusState = typeof BilibiliLoginStatusState[keyof typeof BilibiliLoginStatusState];


export const BilibiliLoginStatusState = {
  generating: 'generating',
  waiting_scan: 'waiting_scan',
  scanned: 'scanned',
  confirmed: 'confirmed',
  logged_in: 'logged_in',
  expired: 'expired',
  failed: 'failed',
  cancelled: 'cancelled',
} as const;
