/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { GatewayProtocol } from './gatewayProtocol';

/**
 * 全部字段可选；省略 api_key 表示保留原 Key，提供则替换。
 */
export interface ClaudeKeyUpdate {
  protocol?: GatewayProtocol;
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
  /** 可选；省略保持不变，空字符串表示清除（回退所选协议的默认地址）。 */
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
