/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface ClaudeKeyCreate {
  /**
     * 完整 Anthropic API Key；仅用于写入，任何响应都不会回读。
     * @minLength 1
     */
  api_key: string;
  /**
     * @minLength 1
     * @maxLength 100
     */
  name: string;
  /**
     * @minLength 1
     * @maxLength 100
     */
  model: string;
  /** 可选；非空必须是 https URL（无 userinfo），存储时去尾 /。 */
  base_url?: string;
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
