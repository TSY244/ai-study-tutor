/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { ClaudeCodeSettingsSavedStatus } from './claudeCodeSettingsSavedStatus';
import type { ClaudeCodeSettingsSavedTarget } from './claudeCodeSettingsSavedTarget';

/**
 * Claude Code 设置已从指定的已保存 Key 应用到运行环境的安全确认。
 */
export interface ClaudeCodeSettingsSaved {
  status: ClaudeCodeSettingsSavedStatus;
  target: ClaudeCodeSettingsSavedTarget;
}
