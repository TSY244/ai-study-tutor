/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface BilibiliCookieUpsert {
  /**
     * Netscape Cookie 导出内容、Cookie 头串（SESSDATA=xxx; ...）或单个 SESSDATA 值；仅用于写入，任何响应都不会回读。
     * @minLength 1
     * @maxLength 65536
     */
  content: string;
}
