/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import { apiFetch } from '../../../http';

export const getDownloadFileUrl = (fileId: string,) => {




  return `/api/v1/files/${fileId}/download`
}

/**
 * @summary 下载已发布文件
 */
export const downloadFile = async (fileId: string, options?: RequestInit): Promise<Blob> => {

  return apiFetch<Blob>(getDownloadFileUrl(fileId),
  {
    ...options,
    method: 'GET'


  }
);}


