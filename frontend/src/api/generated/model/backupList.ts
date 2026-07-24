/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { Backup } from './backup';
import type { PageInfo } from './pageInfo';

export interface BackupList {
  items: Backup[];
  page_info: PageInfo;
}
