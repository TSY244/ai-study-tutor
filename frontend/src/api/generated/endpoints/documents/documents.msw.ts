/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import {
  faker
} from '@faker-js/faker';

import {
  HttpResponse,
  http
} from 'msw';
import type {
  RequestHandlerOptions
} from 'msw';

import {
  DocumentKind,
  JobKind,
  JobStatus
} from '../../model';
import type {
  AsyncAcceptedResponse,
  Document,
  DocumentList,
  DocumentSummaryCollection,
  ProblemResponse
} from '../../model';


export const getListCurrentDocumentsResponseMock = (overrideResponse: Partial<Extract<DocumentSummaryCollection, object>> = {}): DocumentSummaryCollection => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), ...overrideResponse})


export const getListCurrentDocumentsResponseMock200 = (overrideResponse: Partial<Extract<DocumentSummaryCollection, object>> = {}): DocumentSummaryCollection => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), ...overrideResponse})


export const getListCurrentDocumentsResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetCurrentDocumentResponseMock = (overrideResponse: Partial<Extract<Document, object>> = {}): Document => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), content_md: faker.string.alpha({length: {min: 10, max: 20}}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetCurrentDocumentResponseMock200 = (overrideResponse: Partial<Extract<Document, object>> = {}): Document => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), content_md: faker.string.alpha({length: {min: 10, max: 20}}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetCurrentDocumentResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateManualDocumentResponseMock = (overrideResponse: Partial<Extract<Document, object>> = {}): Document => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), content_md: faker.string.alpha({length: {min: 10, max: 20}}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCreateManualDocumentResponseMock201 = (overrideResponse: Partial<Extract<Document, object>> = {}): Document => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), content_md: faker.string.alpha({length: {min: 10, max: 20}}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCreateManualDocumentResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getUpdateManualDocumentResponseMock = (overrideResponse: Partial<Extract<Document, object>> = {}): Document => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), content_md: faker.string.alpha({length: {min: 10, max: 20}}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getUpdateManualDocumentResponseMock200 = (overrideResponse: Partial<Extract<Document, object>> = {}): Document => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), content_md: faker.string.alpha({length: {min: 10, max: 20}}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getUpdateManualDocumentResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateLessonRegenerationResponseMock = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateLessonRegenerationResponseMock202 = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateLessonRegenerationResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getListDocumentVersionsResponseMock = (overrideResponse: Partial<Extract<DocumentList, object>> = {}): DocumentList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListDocumentVersionsResponseMock200 = (overrideResponse: Partial<Extract<DocumentList, object>> = {}): DocumentList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListDocumentVersionsResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetDocumentVersionResponseMock = (overrideResponse: Partial<Extract<Document, object>> = {}): Document => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), content_md: faker.string.alpha({length: {min: 10, max: 20}}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetDocumentVersionResponseMock200 = (overrideResponse: Partial<Extract<Document, object>> = {}): Document => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(DocumentKind)), revision: faker.number.int({min: 1}), content_md: faker.string.alpha({length: {min: 10, max: 20}}), source_job_id: faker.helpers.arrayElement([faker.string.uuid(), null]), is_active: faker.datatype.boolean(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetDocumentVersionResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})



export const getDeleteDocumentResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getListCurrentDocumentsMockHandler = (overrideResponse?: DocumentSummaryCollection | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<DocumentSummaryCollection> | DocumentSummaryCollection), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/documents', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListCurrentDocumentsResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListCurrentDocumentsMockHandler200 = (overrideResponse?: DocumentSummaryCollection | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<DocumentSummaryCollection> | DocumentSummaryCollection), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/documents', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListCurrentDocumentsResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListCurrentDocumentsMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/documents', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListCurrentDocumentsResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetCurrentDocumentMockHandler = (overrideResponse?: Document | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Document> | Document), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/documents/:kind', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetCurrentDocumentResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetCurrentDocumentMockHandler200 = (overrideResponse?: Document | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Document> | Document), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/documents/:kind', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetCurrentDocumentResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetCurrentDocumentMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/documents/:kind', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetCurrentDocumentResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateManualDocumentMockHandler = (overrideResponse?: Document | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Document> | Document), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/documents/:kind', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateManualDocumentResponseMock(),
      { status: 201
      })
  }, options)
}


export const getCreateManualDocumentMockHandler201 = (overrideResponse?: Document | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Document> | Document), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/documents/:kind', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateManualDocumentResponseMock201(),
      { status: 201
      })
  }, options)
}


export const getCreateManualDocumentMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/documents/:kind', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateManualDocumentResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getUpdateManualDocumentMockHandler = (overrideResponse?: Document | ((info: Parameters<Parameters<typeof http.put>[1]>[0]) => Promise<Document> | Document), options?: RequestHandlerOptions) => {
  return http.put('*/api/v1/lessons/:lessonId/documents/:kind', async (info: Parameters<Parameters<typeof http.put>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateManualDocumentResponseMock(),
      { status: 200
      })
  }, options)
}


export const getUpdateManualDocumentMockHandler200 = (overrideResponse?: Document | ((info: Parameters<Parameters<typeof http.put>[1]>[0]) => Promise<Document> | Document), options?: RequestHandlerOptions) => {
  return http.put('*/api/v1/lessons/:lessonId/documents/:kind', async (info: Parameters<Parameters<typeof http.put>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateManualDocumentResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getUpdateManualDocumentMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.put>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.put('*/api/v1/lessons/:lessonId/documents/:kind', async (info: Parameters<Parameters<typeof http.put>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateManualDocumentResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateLessonRegenerationMockHandler = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/regenerations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateLessonRegenerationResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateLessonRegenerationMockHandler202 = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/regenerations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateLessonRegenerationResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateLessonRegenerationMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/regenerations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateLessonRegenerationResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getListDocumentVersionsMockHandler = (overrideResponse?: DocumentList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<DocumentList> | DocumentList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/documents/:kind/versions', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListDocumentVersionsResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListDocumentVersionsMockHandler200 = (overrideResponse?: DocumentList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<DocumentList> | DocumentList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/documents/:kind/versions', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListDocumentVersionsResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListDocumentVersionsMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/documents/:kind/versions', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListDocumentVersionsResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetDocumentVersionMockHandler = (overrideResponse?: Document | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Document> | Document), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/documents/:documentId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetDocumentVersionResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetDocumentVersionMockHandler200 = (overrideResponse?: Document | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Document> | Document), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/documents/:documentId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetDocumentVersionResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetDocumentVersionMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/documents/:documentId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetDocumentVersionResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getDeleteDocumentMockHandler = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/documents/:documentId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteDocumentMockHandler204 = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/documents/:documentId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteDocumentMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/documents/:documentId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getDeleteDocumentResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getDocumentsMock = () => [
  getListCurrentDocumentsMockHandler(),
  getGetCurrentDocumentMockHandler(),
  getCreateManualDocumentMockHandler(),
  getUpdateManualDocumentMockHandler(),
  getCreateLessonRegenerationMockHandler(),
  getListDocumentVersionsMockHandler(),
  getGetDocumentVersionMockHandler(),
  getDeleteDocumentMockHandler()
]
