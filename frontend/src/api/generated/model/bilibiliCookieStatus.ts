/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

/**
 * B 站 Cookie 的安全视图；绝不包含完整 Cookie。
 */
export interface BilibiliCookieStatus {
  configured: boolean;
  /**
     * Cookie 文件最后保存时间；未配置时为 null。
     * @nullable
     */
  updated_at: string | null;
  /**
     * SESSDATA 本地有效期；会话级（无 expires）时为 null。
     * @nullable
     */
  sessdata_expires_at: string | null;
  /** SESSDATA 是否已超过本地有效期（以 B 站实际登录状态为准，检测任务可确认）。 */
  expired: boolean;
  /** 脱敏 SESSDATA（前 7 位 + 后 4 位）；未配置时为空串。 */
  masked_sessdata: string;
  /**
     * 保存的 bilibili.com 域名 Cookie 条目数。
     * @minimum 0
     */
  cookie_count: number;
}
