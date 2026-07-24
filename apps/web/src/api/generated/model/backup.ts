/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { BackupStatus } from './backupStatus';

export interface Backup {
  id: string;
  status: BackupStatus;
  application_version: string;
  database_version: string;
  /**
     * @minimum 0
     * @nullable
     */
  size_bytes: number | null;
  /**
     * backups 目录内的安全展示名称，不是宿主机绝对路径或在线下载 URL。
     * @nullable
     */
  archive_name?: string | null;
  created_at: string;
  /** @nullable */
  completed_at?: string | null;
}
