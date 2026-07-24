/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  AsyncAcceptedResponse,
  Document,
  DocumentKind,
  DocumentList,
  DocumentSummaryCollection,
  ListDocumentVersionsParams,
  RegenerationCreate
} from '../../model';

import { apiFetch } from '../../../http';

export const getListCurrentDocumentsUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/documents`
}

/**
 * @summary 查询课时当前文档
 */
export const listCurrentDocuments = async (lessonId: string, options?: RequestInit): Promise<DocumentSummaryCollection> => {

  return apiFetch<DocumentSummaryCollection>(getListCurrentDocumentsUrl(lessonId),
  {
    ...options,
    method: 'GET'


  }
);}


export const getGetCurrentDocumentUrl = (lessonId: string,
    kind: DocumentKind,) => {




  return `/api/v1/lessons/${lessonId}/documents/${kind}`
}

/**
 * @summary 查询指定类型的当前文档
 */
export const getCurrentDocument = async (lessonId: string,
    kind: DocumentKind, options?: RequestInit): Promise<Document> => {

  return apiFetch<Document>(getGetCurrentDocumentUrl(lessonId,kind),
  {
    ...options,
    method: 'GET'


  }
);}


export const getCreateLessonRegenerationUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/regenerations`
}

/**
 * @summary 创建笔记重生成任务
 */
export const createLessonRegeneration = async (lessonId: string,
    regenerationCreate: RegenerationCreate, options?: RequestInit): Promise<AsyncAcceptedResponse> => {

  return apiFetch<AsyncAcceptedResponse>(getCreateLessonRegenerationUrl(lessonId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(regenerationCreate)
  }
);}


export const getListDocumentVersionsUrl = (lessonId: string,
    kind: DocumentKind,
    params?: ListDocumentVersionsParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/lessons/${lessonId}/documents/${kind}/versions?${stringifiedParams}` : `/api/v1/lessons/${lessonId}/documents/${kind}/versions`
}

/**
 * @summary 查询指定类型的文档历史版本
 */
export const listDocumentVersions = async (lessonId: string,
    kind: DocumentKind,
    params?: ListDocumentVersionsParams, options?: RequestInit): Promise<DocumentList> => {

  return apiFetch<DocumentList>(getListDocumentVersionsUrl(lessonId,kind,params),
  {
    ...options,
    method: 'GET'


  }
);}


export const getGetDocumentVersionUrl = (documentId: string,) => {




  return `/api/v1/documents/${documentId}`
}

/**
 * @summary 查询指定文档版本
 */
export const getDocumentVersion = async (documentId: string, options?: RequestInit): Promise<Document> => {

  return apiFetch<Document>(getGetDocumentVersionUrl(documentId),
  {
    ...options,
    method: 'GET'


  }
);}


