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
  JobKind,
  JobStatus,
  MaterialKind
} from '../../model';
import type {
  MaterialJobAccepted,
  MaterialList,
  ProblemResponse
} from '../../model';


export const getCreateUrlMaterialResponseMock = (overrideResponse: Partial<Extract<MaterialJobAccepted, object>> = {}): MaterialJobAccepted => ({material: {id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(MaterialKind)), status: faker.helpers.arrayElement(['pending','ready','failed'] as const), source_url: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.internet.url(), null]), undefined]), original_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateUrlMaterialResponseMock202 = (overrideResponse: Partial<Extract<MaterialJobAccepted, object>> = {}): MaterialJobAccepted => ({material: {id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(MaterialKind)), status: faker.helpers.arrayElement(['pending','ready','failed'] as const), source_url: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.internet.url(), null]), undefined]), original_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateUrlMaterialResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateTextMaterialResponseMock = (overrideResponse: Partial<Extract<MaterialJobAccepted, object>> = {}): MaterialJobAccepted => ({material: {id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(MaterialKind)), status: faker.helpers.arrayElement(['pending','ready','failed'] as const), source_url: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.internet.url(), null]), undefined]), original_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateTextMaterialResponseMock202 = (overrideResponse: Partial<Extract<MaterialJobAccepted, object>> = {}): MaterialJobAccepted => ({material: {id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(MaterialKind)), status: faker.helpers.arrayElement(['pending','ready','failed'] as const), source_url: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.internet.url(), null]), undefined]), original_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateTextMaterialResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getUploadMaterialResponseMock = (overrideResponse: Partial<Extract<MaterialJobAccepted, object>> = {}): MaterialJobAccepted => ({material: {id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(MaterialKind)), status: faker.helpers.arrayElement(['pending','ready','failed'] as const), source_url: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.internet.url(), null]), undefined]), original_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getUploadMaterialResponseMock202 = (overrideResponse: Partial<Extract<MaterialJobAccepted, object>> = {}): MaterialJobAccepted => ({material: {id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(MaterialKind)), status: faker.helpers.arrayElement(['pending','ready','failed'] as const), source_url: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.internet.url(), null]), undefined]), original_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getUploadMaterialResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getListMaterialsResponseMock = (overrideResponse: Partial<Extract<MaterialList, object>> = {}): MaterialList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(MaterialKind)), status: faker.helpers.arrayElement(['pending','ready','failed'] as const), source_url: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.internet.url(), null]), undefined]), original_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListMaterialsResponseMock200 = (overrideResponse: Partial<Extract<MaterialList, object>> = {}): MaterialList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(MaterialKind)), status: faker.helpers.arrayElement(['pending','ready','failed'] as const), source_url: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.internet.url(), null]), undefined]), original_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListMaterialsResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})



export const getDeleteMaterialResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getCreateUrlMaterialMockHandler = (overrideResponse?: MaterialJobAccepted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<MaterialJobAccepted> | MaterialJobAccepted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/materials/url', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateUrlMaterialResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateUrlMaterialMockHandler202 = (overrideResponse?: MaterialJobAccepted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<MaterialJobAccepted> | MaterialJobAccepted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/materials/url', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateUrlMaterialResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateUrlMaterialMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/materials/url', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateUrlMaterialResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateTextMaterialMockHandler = (overrideResponse?: MaterialJobAccepted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<MaterialJobAccepted> | MaterialJobAccepted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/materials/text', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateTextMaterialResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateTextMaterialMockHandler202 = (overrideResponse?: MaterialJobAccepted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<MaterialJobAccepted> | MaterialJobAccepted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/materials/text', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateTextMaterialResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateTextMaterialMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/materials/text', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateTextMaterialResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getUploadMaterialMockHandler = (overrideResponse?: MaterialJobAccepted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<MaterialJobAccepted> | MaterialJobAccepted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/materials/upload', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUploadMaterialResponseMock(),
      { status: 202
      })
  }, options)
}


export const getUploadMaterialMockHandler202 = (overrideResponse?: MaterialJobAccepted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<MaterialJobAccepted> | MaterialJobAccepted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/materials/upload', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUploadMaterialResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getUploadMaterialMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/materials/upload', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUploadMaterialResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getListMaterialsMockHandler = (overrideResponse?: MaterialList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<MaterialList> | MaterialList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/materials', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListMaterialsResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListMaterialsMockHandler200 = (overrideResponse?: MaterialList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<MaterialList> | MaterialList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/materials', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListMaterialsResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListMaterialsMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/materials', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListMaterialsResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getDeleteMaterialMockHandler = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/materials/:materialId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteMaterialMockHandler204 = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/materials/:materialId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteMaterialMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/materials/:materialId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getDeleteMaterialResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getMaterialsMock = () => [
  getCreateUrlMaterialMockHandler(),
  getCreateTextMaterialMockHandler(),
  getUploadMaterialMockHandler(),
  getListMaterialsMockHandler(),
  getDeleteMaterialMockHandler()
]
