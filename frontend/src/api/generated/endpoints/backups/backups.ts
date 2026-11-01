/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  BackupList,
  ListBackupsParams
} from '../../model';

import { apiFetch } from '../../../http';

export const getListBackupsUrl = (params?: ListBackupsParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/backups?${stringifiedParams}` : `/api/v1/backups`
}

/**
 * @summary 查询备份包
 */
export const listBackups = async (params?: ListBackupsParams, options?: RequestInit): Promise<BackupList> => {

  return apiFetch<BackupList>(getListBackupsUrl(params),
  {
    ...options,
    method: 'GET'


  }
);}


