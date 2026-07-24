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
  QuestionType
} from '../../model';
import type {
  AsyncAcceptedResponse,
  ProblemResponse,
  Quiz,
  QuizList
} from '../../model';


export const getCreateQuizGenerationResponseMock = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateQuizGenerationResponseMock202 = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateQuizGenerationResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateReviewQuizGenerationResponseMock = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateReviewQuizGenerationResponseMock202 = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateReviewQuizGenerationResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getListQuizzesResponseMock = (overrideResponse: Partial<Extract<QuizList, object>> = {}): QuizList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), source: faker.helpers.arrayElement(['regular','retest','assembled'] as const), question_count: faker.number.int({min: 0}), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListQuizzesResponseMock200 = (overrideResponse: Partial<Extract<QuizList, object>> = {}): QuizList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), source: faker.helpers.arrayElement(['regular','retest','assembled'] as const), question_count: faker.number.int({min: 0}), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListQuizzesResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetQuizResponseMock = (overrideResponse: Partial<Extract<Quiz, object>> = {}): Quiz => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), source: faker.helpers.arrayElement(['regular','retest','assembled'] as const), status: faker.helpers.arrayElement(['draft','ready','archived'] as const), questions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), type: faker.helpers.arrayElement(Object.values(QuestionType)), stem: faker.string.alpha({length: {min: 10, max: 20}}), options: faker.helpers.arrayElement([faker.helpers.arrayElement([{
        [faker.string.alphanumeric(5)]: faker.string.alpha({length: {min: 10, max: 20}})
      },null,]), null]), tested_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), difficulty: faker.helpers.arrayElement(['easy','medium','hard','challenge'] as const), cognitive_level: faker.helpers.arrayElement(['recall','explain','apply','transfer','reflect'] as const), score: faker.number.float({min: 0, fractionDigits: 2})})), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetQuizResponseMock200 = (overrideResponse: Partial<Extract<Quiz, object>> = {}): Quiz => ({id: faker.string.uuid(), lesson_id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), source: faker.helpers.arrayElement(['regular','retest','assembled'] as const), status: faker.helpers.arrayElement(['draft','ready','archived'] as const), questions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), type: faker.helpers.arrayElement(Object.values(QuestionType)), stem: faker.string.alpha({length: {min: 10, max: 20}}), options: faker.helpers.arrayElement([faker.helpers.arrayElement([{
        [faker.string.alphanumeric(5)]: faker.string.alpha({length: {min: 10, max: 20}})
      },null,]), null]), tested_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), difficulty: faker.helpers.arrayElement(['easy','medium','hard','challenge'] as const), cognitive_level: faker.helpers.arrayElement(['recall','explain','apply','transfer','reflect'] as const), score: faker.number.float({min: 0, fractionDigits: 2})})), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetQuizResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getCreateQuizGenerationMockHandler = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/quiz-generations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateQuizGenerationResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateQuizGenerationMockHandler202 = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/quiz-generations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateQuizGenerationResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateQuizGenerationMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/quiz-generations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateQuizGenerationResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateReviewQuizGenerationMockHandler = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/review-quiz-generations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateReviewQuizGenerationResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateReviewQuizGenerationMockHandler202 = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/review-quiz-generations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateReviewQuizGenerationResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateReviewQuizGenerationMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons/:lessonId/review-quiz-generations', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateReviewQuizGenerationResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getListQuizzesMockHandler = (overrideResponse?: QuizList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<QuizList> | QuizList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/quizzes', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListQuizzesResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListQuizzesMockHandler200 = (overrideResponse?: QuizList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<QuizList> | QuizList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/quizzes', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListQuizzesResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListQuizzesMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/quizzes', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListQuizzesResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetQuizMockHandler = (overrideResponse?: Quiz | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Quiz> | Quiz), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/quizzes/:quizId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetQuizResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetQuizMockHandler200 = (overrideResponse?: Quiz | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Quiz> | Quiz), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/quizzes/:quizId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetQuizResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetQuizMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/quizzes/:quizId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetQuizResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getQuizzesMock = () => [
  getCreateQuizGenerationMockHandler(),
  getCreateReviewQuizGenerationMockHandler(),
  getListQuizzesMockHandler(),
  getGetQuizMockHandler()
]
