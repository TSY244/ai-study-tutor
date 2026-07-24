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

import type {
  ProblemResponse,
  Progress
} from '../../model';


export const getGetCourseProgressResponseMock = (overrideResponse: Partial<Extract<Progress, object>> = {}): Progress => ({scope_type: faker.helpers.arrayElement(['course','lesson'] as const), scope_id: faker.string.uuid(), mastery: faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), formula_version: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), completed_attempts: faker.number.int({min: 0}), active_weaknesses: faker.number.int({min: 0}), trend: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({recorded_at: faker.date.past().toISOString().slice(0, 19) + 'Z', mastery: faker.number.float({min: 0, max: 1, fractionDigits: 2}), formula_version: faker.string.alpha({length: {min: 10, max: 20}}), source_attempt_id: faker.helpers.arrayElement([faker.string.uuid(), null])})), ...overrideResponse})


export const getGetCourseProgressResponseMock200 = (overrideResponse: Partial<Extract<Progress, object>> = {}): Progress => ({scope_type: faker.helpers.arrayElement(['course','lesson'] as const), scope_id: faker.string.uuid(), mastery: faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), formula_version: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), completed_attempts: faker.number.int({min: 0}), active_weaknesses: faker.number.int({min: 0}), trend: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({recorded_at: faker.date.past().toISOString().slice(0, 19) + 'Z', mastery: faker.number.float({min: 0, max: 1, fractionDigits: 2}), formula_version: faker.string.alpha({length: {min: 10, max: 20}}), source_attempt_id: faker.helpers.arrayElement([faker.string.uuid(), null])})), ...overrideResponse})


export const getGetCourseProgressResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetLessonProgressResponseMock = (overrideResponse: Partial<Extract<Progress, object>> = {}): Progress => ({scope_type: faker.helpers.arrayElement(['course','lesson'] as const), scope_id: faker.string.uuid(), mastery: faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), formula_version: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), completed_attempts: faker.number.int({min: 0}), active_weaknesses: faker.number.int({min: 0}), trend: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({recorded_at: faker.date.past().toISOString().slice(0, 19) + 'Z', mastery: faker.number.float({min: 0, max: 1, fractionDigits: 2}), formula_version: faker.string.alpha({length: {min: 10, max: 20}}), source_attempt_id: faker.helpers.arrayElement([faker.string.uuid(), null])})), ...overrideResponse})


export const getGetLessonProgressResponseMock200 = (overrideResponse: Partial<Extract<Progress, object>> = {}): Progress => ({scope_type: faker.helpers.arrayElement(['course','lesson'] as const), scope_id: faker.string.uuid(), mastery: faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), formula_version: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), completed_attempts: faker.number.int({min: 0}), active_weaknesses: faker.number.int({min: 0}), trend: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({recorded_at: faker.date.past().toISOString().slice(0, 19) + 'Z', mastery: faker.number.float({min: 0, max: 1, fractionDigits: 2}), formula_version: faker.string.alpha({length: {min: 10, max: 20}}), source_attempt_id: faker.helpers.arrayElement([faker.string.uuid(), null])})), ...overrideResponse})


export const getGetLessonProgressResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getGetCourseProgressMockHandler = (overrideResponse?: Progress | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Progress> | Progress), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses/:courseId/progress', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetCourseProgressResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetCourseProgressMockHandler200 = (overrideResponse?: Progress | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Progress> | Progress), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses/:courseId/progress', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetCourseProgressResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetCourseProgressMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses/:courseId/progress', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetCourseProgressResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetLessonProgressMockHandler = (overrideResponse?: Progress | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Progress> | Progress), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/progress', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetLessonProgressResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetLessonProgressMockHandler200 = (overrideResponse?: Progress | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Progress> | Progress), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/progress', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetLessonProgressResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetLessonProgressMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/progress', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetLessonProgressResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getProgressMock = () => [
  getGetCourseProgressMockHandler(),
  getGetLessonProgressMockHandler()
]
