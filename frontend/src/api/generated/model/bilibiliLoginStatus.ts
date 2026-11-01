/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { BilibiliCookieStatus } from './bilibiliCookieStatus';
import type { BilibiliLoginStatusState } from './bilibiliLoginStatusState';

export interface BilibiliLoginStatus {
  state: BilibiliLoginStatusState;
  message: string;
  expires_at?: string;
  /** 登录二维码 PNG 的 data URL；不包含登录凭证 */
  qr_image_data_url?: string;
  cookie?: BilibiliCookieStatus;
}
