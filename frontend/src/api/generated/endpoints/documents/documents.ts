/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  AsyncAcceptedResponse,
  Document,
  DocumentCreate,
  DocumentKind,
  DocumentList,
  DocumentSummaryCollection,
  DocumentUpdate,
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


export const getCreateManualDocumentUrl = (lessonId: string,
    kind: 'raw_text' | 'student_notes' | 'teacher_outline',) => {




  return `/api/v1/lessons/${lessonId}/documents/${kind}`
}

/**
 * 该类型已有当前文档时返回 409；支持原文、学生笔记和教师大纲。
 * @summary 手工新增课时文档
 */
export const createManualDocument = async (lessonId: string,
    kind: 'raw_text' | 'student_notes' | 'teacher_outline',
    documentCreate: DocumentCreate, options?: RequestInit): Promise<Document> => {

  return apiFetch<Document>(getCreateManualDocumentUrl(lessonId,kind),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(documentCreate)
  }
);}


export const getUpdateManualDocumentUrl = (lessonId: string,
    kind: 'raw_text' | 'student_notes' | 'teacher_outline',) => {




  return `/api/v1/lessons/${lessonId}/documents/${kind}`
}

/**
 * expected_revision 必须等于当前版本。旧版本保留；修改原文会停用旧知识点。
 * @summary 编辑文档并发布新版本
 */
export const updateManualDocument = async (lessonId: string,
    kind: 'raw_text' | 'student_notes' | 'teacher_outline',
    documentUpdate: DocumentUpdate, options?: RequestInit): Promise<Document> => {

  return apiFetch<Document>(getUpdateManualDocumentUrl(lessonId,kind),
  {
    ...options,
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(documentUpdate)
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


export const getDeleteDocumentUrl = (documentId: string,) => {




  return `/api/v1/documents/${documentId}`
}

/**
 * 删除当前版本后该类型暂无当前文档，不自动回退历史版本。
 * @summary 删除指定文档版本
 */
export const deleteDocument = async (documentId: string, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteDocumentUrl(documentId),
  {
    ...options,
    method: 'DELETE'


  }
);}


