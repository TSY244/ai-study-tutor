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
  Health,
  ProblemResponse,
  ServiceUnavailableResponse,
  SystemStatus
} from '../../model';


export const getGetHealthResponseMock = (overrideResponse: Partial<Extract<Health, object>> = {}): Health => ({status: faker.helpers.arrayElement(['ok'] as const), ...overrideResponse})


export const getGetHealthResponseMock200 = (overrideResponse: Partial<Extract<Health, object>> = {}): Health => ({status: faker.helpers.arrayElement(['ok'] as const), ...overrideResponse})

export const getGetReadinessResponseMock = (overrideResponse: Partial<Extract<Health, object>> = {}): Health => ({status: faker.helpers.arrayElement(['ok'] as const), ...overrideResponse})


export const getGetReadinessResponseMock200 = (overrideResponse: Partial<Extract<Health, object>> = {}): Health => ({status: faker.helpers.arrayElement(['ok'] as const), ...overrideResponse})


export const getGetReadinessResponseMock503 = (overrideResponse: Partial<Extract<ServiceUnavailableResponse, object>> = {}): ServiceUnavailableResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetSystemStatusResponseMock = (overrideResponse: Partial<Extract<SystemStatus, object>> = {}): SystemStatus => ({status: faker.helpers.arrayElement(['ok','degraded','unavailable'] as const), version: faker.string.alpha({length: {min: 10, max: 20}}), maintenance: {enabled: faker.datatype.boolean(), reason: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null])}, database: {status: faker.helpers.arrayElement(['ok','degraded','unavailable','unknown'] as const), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined])}, worker: {status: faker.helpers.arrayElement(['ok','degraded','unavailable','unknown'] as const), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined])}, claude: {status: faker.helpers.arrayElement(['ok','degraded','unavailable','unknown'] as const), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined])}, storage: {status: faker.helpers.arrayElement(['ok','degraded','unavailable','unknown'] as const), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined])}, ...overrideResponse})


export const getGetSystemStatusResponseMock200 = (overrideResponse: Partial<Extract<SystemStatus, object>> = {}): SystemStatus => ({status: faker.helpers.arrayElement(['ok','degraded','unavailable'] as const), version: faker.string.alpha({length: {min: 10, max: 20}}), maintenance: {enabled: faker.datatype.boolean(), reason: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null])}, database: {status: faker.helpers.arrayElement(['ok','degraded','unavailable','unknown'] as const), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined])}, worker: {status: faker.helpers.arrayElement(['ok','degraded','unavailable','unknown'] as const), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined])}, claude: {status: faker.helpers.arrayElement(['ok','degraded','unavailable','unknown'] as const), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined])}, storage: {status: faker.helpers.arrayElement(['ok','degraded','unavailable','unknown'] as const), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined])}, ...overrideResponse})


export const getGetSystemStatusResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateClaudeConnectivityTestResponseMock = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateClaudeConnectivityTestResponseMock202 = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateClaudeConnectivityTestResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getGetHealthMockHandler = (overrideResponse?: Health | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Health> | Health), options?: RequestHandlerOptions) => {
  return http.get('*/healthz', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetHealthResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetHealthMockHandler200 = (overrideResponse?: Health | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Health> | Health), options?: RequestHandlerOptions) => {
  return http.get('*/healthz', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetHealthResponseMock200(),
      { status: 200
      })
  }, options)
}

export const getGetReadinessMockHandler = (overrideResponse?: Health | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Health> | Health), options?: RequestHandlerOptions) => {
  return http.get('*/readyz', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetReadinessResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetReadinessMockHandler200 = (overrideResponse?: Health | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Health> | Health), options?: RequestHandlerOptions) => {
  return http.get('*/readyz', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetReadinessResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetReadinessMockHandler503 = (overrideResponse?: ServiceUnavailableResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ServiceUnavailableResponse> | ServiceUnavailableResponse), options?: RequestHandlerOptions) => {
  return http.get('*/readyz', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetReadinessResponseMock503(),
      { status: 503
      })
  }, options)
}

export const getGetSystemStatusMockHandler = (overrideResponse?: SystemStatus | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<SystemStatus> | SystemStatus), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/system/status', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetSystemStatusResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetSystemStatusMockHandler200 = (overrideResponse?: SystemStatus | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<SystemStatus> | SystemStatus), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/system/status', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetSystemStatusResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetSystemStatusMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/system/status', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetSystemStatusResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateClaudeConnectivityTestMockHandler = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-tests', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateClaudeConnectivityTestResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateClaudeConnectivityTestMockHandler202 = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-tests', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateClaudeConnectivityTestResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateClaudeConnectivityTestMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-tests', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateClaudeConnectivityTestResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getSystemMock = () => [
  getGetHealthMockHandler(),
  getGetReadinessMockHandler(),
  getGetSystemStatusMockHandler(),
  getCreateClaudeConnectivityTestMockHandler()
]
