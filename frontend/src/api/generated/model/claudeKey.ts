/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { ClaudeKeyStatus } from './claudeKeyStatus';
import type { GatewayProtocol } from './gatewayProtocol';

/**
 * Claude 网关密钥的安全视图；绝不包含完整 api_key。
 */
export interface ClaudeKey {
  protocol: GatewayProtocol;
  id: string;
  name: string;
  /** 脱敏展示形式（如 sk-ant-...WXYZ）；无法反推完整 Key。 */
  masked_key: string;
  /** Key 的短指纹（sha256: 前缀），用于区分密钥而不泄露内容。 */
  fingerprint: string;
  model: string;
  /** 该 Key 专属的 API base URL；留空时 Anthropic 使用部署默认值，OpenAI 使用 https://api.openai.com/v1。 */
  base_url?: string;
  enabled: boolean;
  /** @minimum 0 */
  priority: number;
  /** @minimum 1 */
  weight: number;
  /**
     * @minimum 1
     * @maximum 3600
     */
  timeout_seconds: number;
  /**
     * @minimum 1
     * @maximum 86400
     */
  cooldown_seconds: number;
  /**
     * @minimum 1
     * @maximum 100
     */
  max_consecutive_failures: number;
  status: ClaudeKeyStatus;
  /** @minimum 0 */
  consecutive_failures: number;
  /** @nullable */
  last_error_code: string | null;
  /** @nullable */
  last_success_at: string | null;
  /** @nullable */
  last_failure_at: string | null;
  /** @nullable */
  cooldown_until: string | null;
  /**
     * @minimum 0
     * @nullable
     */
  last_latency_ms: number | null;
  created_at: string;
  updated_at: string;
}
