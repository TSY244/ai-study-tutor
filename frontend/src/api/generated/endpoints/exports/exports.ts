/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  AsyncAcceptedResponse,
  ExportCreate,
  FileObject
} from '../../model';

import { apiFetch } from '../../../http';

export const getCreateLessonExportUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/exports`
}

/**
 * @summary 创建课时导出
 */
export const createLessonExport = async (lessonId: string,
    exportCreate: ExportCreate, options?: RequestInit): Promise<FileObject | AsyncAcceptedResponse> => {

  return apiFetch<FileObject | AsyncAcceptedResponse>(getCreateLessonExportUrl(lessonId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(exportCreate)
  }
);}


