/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

/**
 * 全部字段可选；省略 api_key 表示保留原 Key，提供则替换。
 */
export interface ClaudeKeyUpdate {
  /**
     * 提供时替换完整 Key；任何响应都不会回读。
     * @minLength 1
     */
  api_key?: string;
  /**
     * @minLength 1
     * @maxLength 100
     */
  name?: string;
  /**
     * @minLength 1
     * @maxLength 100
     */
  model?: string;
  enabled?: boolean;
  /** @minimum 0 */
  priority?: number;
  /**
     * @minimum 1
     * @maximum 1000
     */
  weight?: number;
  /**
     * @minimum 1
     * @maximum 3600
     */
  timeout_seconds?: number;
  /**
     * @minimum 1
     * @maximum 86400
     */
  cooldown_seconds?: number;
  /**
     * @minimum 1
     * @maximum 100
     */
  max_consecutive_failures?: number;
}
