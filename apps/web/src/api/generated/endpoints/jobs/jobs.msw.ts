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
  Job,
  JobEventList,
  JobList,
  ProblemResponse
} from '../../model';


export const getListJobsResponseMock = (overrideResponse: Partial<Extract<JobList, object>> = {}): JobList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListJobsResponseMock200 = (overrideResponse: Partial<Extract<JobList, object>> = {}): JobList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListJobsResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetJobResponseMock = (overrideResponse: Partial<Extract<Job, object>> = {}): Job => ({id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetJobResponseMock200 = (overrideResponse: Partial<Extract<Job, object>> = {}): Job => ({id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetJobResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCancelJobResponseMock = (overrideResponse: Partial<Extract<Job, object>> = {}): Job => ({id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCancelJobResponseMock202 = (overrideResponse: Partial<Extract<Job, object>> = {}): Job => ({id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCancelJobResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getRetryJobResponseMock = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getRetryJobResponseMock202 = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getRetryJobResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getListJobEventsResponseMock = (overrideResponse: Partial<Extract<JobEventList, object>> = {}): JobEventList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.number.int({min: 1}), job_id: faker.string.uuid(), event_type: faker.helpers.arrayElement(['job.queued','job.started','job.progress','job.validating','job.cancel_requested','job.succeeded','job.failed','job.cancelled'] as const), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), data: faker.helpers.arrayElement([{}, undefined]), occurred_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListJobEventsResponseMock200 = (overrideResponse: Partial<Extract<JobEventList, object>> = {}): JobEventList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.number.int({min: 1}), job_id: faker.string.uuid(), event_type: faker.helpers.arrayElement(['job.queued','job.started','job.progress','job.validating','job.cancel_requested','job.succeeded','job.failed','job.cancelled'] as const), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), data: faker.helpers.arrayElement([{}, undefined]), occurred_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListJobEventsResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getListJobsMockHandler = (overrideResponse?: JobList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<JobList> | JobList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/jobs', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListJobsResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListJobsMockHandler200 = (overrideResponse?: JobList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<JobList> | JobList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/jobs', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListJobsResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListJobsMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/jobs', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListJobsResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetJobMockHandler = (overrideResponse?: Job | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Job> | Job), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/jobs/:jobId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetJobResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetJobMockHandler200 = (overrideResponse?: Job | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Job> | Job), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/jobs/:jobId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetJobResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetJobMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/jobs/:jobId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetJobResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCancelJobMockHandler = (overrideResponse?: Job | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Job> | Job), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/jobs/:jobId/cancel', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCancelJobResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCancelJobMockHandler202 = (overrideResponse?: Job | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Job> | Job), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/jobs/:jobId/cancel', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCancelJobResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCancelJobMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/jobs/:jobId/cancel', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCancelJobResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getRetryJobMockHandler = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/jobs/:jobId/retry', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getRetryJobResponseMock(),
      { status: 202
      })
  }, options)
}


export const getRetryJobMockHandler202 = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/jobs/:jobId/retry', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getRetryJobResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getRetryJobMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/jobs/:jobId/retry', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getRetryJobResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getListJobEventsMockHandler = (overrideResponse?: JobEventList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<JobEventList> | JobEventList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/jobs/:jobId/events', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListJobEventsResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListJobEventsMockHandler200 = (overrideResponse?: JobEventList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<JobEventList> | JobEventList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/jobs/:jobId/events', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListJobEventsResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListJobEventsMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/jobs/:jobId/events', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListJobEventsResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getJobsMock = () => [
  getListJobsMockHandler(),
  getGetJobMockHandler(),
  getCancelJobMockHandler(),
  getRetryJobMockHandler(),
  getListJobEventsMockHandler()
]
