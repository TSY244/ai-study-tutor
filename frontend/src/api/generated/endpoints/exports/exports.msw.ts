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
  JobStatus
} from '../../model';
import type {
  AsyncAcceptedResponse,
  FileObject,
  ProblemResponse
} from '../../model';


export const getCreateLessonExportResponseMock = (overrideResponse: Partial<Extract<FileObject | AsyncAcceptedResponse, object>> = {}): FileObject | AsyncAcceptedResponse => (faker.helpers.arrayElement([{id: faker.string.uuid(), display_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), media_type: faker.string.alpha({length: {min: 10, max: 20}}), size_bytes: faker.number.int({min: 0}), sha256: faker.helpers.fromRegExp("^[a-f0-9]{64}$"), download_url: faker.string.alpha({length: {min: 10, max: 20}}), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse}, {job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse}]))


export const getCreateLessonExportResponseMock200 = (overrideResponse: Partial<Extract<FileObject, object>> = {}): FileObject => ({id: faker.string.uuid(), display_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), media_type: faker.string.alpha({length: {min: 10, max: 20}}), size_bytes: faker.number.int({min: 0}), sha256: faker.helpers.fromRegExp("^[a-f0-9]{64}$"), download_url: faker.string.alpha({length: {min: 10, max: 20}}), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCreateLessonExportResponseMock202 = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateLessonExportResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getCreateLessonExportMockHandler = (overrideResponse?: FileObject | AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<FileObject | AsyncAcceptedResponse> | FileObject | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/exports', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateLessonExportResponseMock(),
      { status: 200
      })
  }, options)
}


export const getCreateLessonExportMockHandler200 = (overrideResponse?: FileObject | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<FileObject> | FileObject), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/exports', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateLessonExportResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getCreateLessonExportMockHandler202 = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/exports', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateLessonExportResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateLessonExportMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/exports', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateLessonExportResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getExportsMock = () => [
  getCreateLessonExportMockHandler()
]
