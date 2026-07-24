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
  SummaryEvaluationStatus
} from '../../model';
import type {
  ProblemResponse,
  SummaryEvaluation,
  SummaryEvaluationAccepted
} from '../../model';


export const getCreateSummaryEvaluationResponseMock = (overrideResponse: Partial<Extract<SummaryEvaluationAccepted, object>> = {}): SummaryEvaluationAccepted => ({evaluation: {id: faker.string.uuid(), lesson_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(SummaryEvaluationStatus)), coverage_score: faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), correct_points: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), misconceptions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), missing_points: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), suggestions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined])}, job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateSummaryEvaluationResponseMock202 = (overrideResponse: Partial<Extract<SummaryEvaluationAccepted, object>> = {}): SummaryEvaluationAccepted => ({evaluation: {id: faker.string.uuid(), lesson_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(SummaryEvaluationStatus)), coverage_score: faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), correct_points: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), misconceptions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), missing_points: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), suggestions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined])}, job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateSummaryEvaluationResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetSummaryEvaluationResponseMock = (overrideResponse: Partial<Extract<SummaryEvaluation, object>> = {}): SummaryEvaluation => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(SummaryEvaluationStatus)), coverage_score: faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), correct_points: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), misconceptions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), missing_points: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), suggestions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), ...overrideResponse})


export const getGetSummaryEvaluationResponseMock200 = (overrideResponse: Partial<Extract<SummaryEvaluation, object>> = {}): SummaryEvaluation => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(SummaryEvaluationStatus)), coverage_score: faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), correct_points: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), misconceptions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), missing_points: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), suggestions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), ...overrideResponse})


export const getGetSummaryEvaluationResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getCreateSummaryEvaluationMockHandler = (overrideResponse?: SummaryEvaluationAccepted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<SummaryEvaluationAccepted> | SummaryEvaluationAccepted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/summary-evaluations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateSummaryEvaluationResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateSummaryEvaluationMockHandler202 = (overrideResponse?: SummaryEvaluationAccepted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<SummaryEvaluationAccepted> | SummaryEvaluationAccepted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/summary-evaluations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateSummaryEvaluationResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateSummaryEvaluationMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/summary-evaluations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateSummaryEvaluationResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetSummaryEvaluationMockHandler = (overrideResponse?: SummaryEvaluation | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<SummaryEvaluation> | SummaryEvaluation), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/summary-evaluations/:evaluationId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetSummaryEvaluationResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetSummaryEvaluationMockHandler200 = (overrideResponse?: SummaryEvaluation | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<SummaryEvaluation> | SummaryEvaluation), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/summary-evaluations/:evaluationId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetSummaryEvaluationResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetSummaryEvaluationMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/summary-evaluations/:evaluationId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetSummaryEvaluationResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getSummariesMock = () => [
  getCreateSummaryEvaluationMockHandler(),
  getGetSummaryEvaluationMockHandler()
]
