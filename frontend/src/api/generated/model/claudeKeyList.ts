/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { ClaudeKey } from './claudeKey';
import type { ClaudeKeyListRouting } from './claudeKeyListRouting';

export interface ClaudeKeyList {
  items: ClaudeKey[];
  routing: ClaudeKeyListRouting;
  /** keys.json 无启用 Key 且 Worker 仍通过旧 ANTHROPIC_API_KEY_FILE 单 Key 工作时为 true。 */
  legacy_fallback: boolean;
  write_enabled: boolean;
}
