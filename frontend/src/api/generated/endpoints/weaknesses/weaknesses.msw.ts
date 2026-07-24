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
  Weakness,
  WeaknessList
} from '../../model';


export const getListWeaknessesResponseMock = (overrideResponse: Partial<Extract<WeaknessList, object>> = {}): WeaknessList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), course_id: faker.string.uuid(), lesson_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), concept_key: faker.string.alpha({length: {min: 10, max: 20}}), concept_label: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), evidence: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({})), severity: faker.number.int({min: 1, max: 5}), review_count: faker.number.int({min: 0}), mastered: faker.datatype.boolean(), version: faker.number.int(), first_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', last_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListWeaknessesResponseMock200 = (overrideResponse: Partial<Extract<WeaknessList, object>> = {}): WeaknessList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), course_id: faker.string.uuid(), lesson_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), concept_key: faker.string.alpha({length: {min: 10, max: 20}}), concept_label: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), evidence: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({})), severity: faker.number.int({min: 1, max: 5}), review_count: faker.number.int({min: 0}), mastered: faker.datatype.boolean(), version: faker.number.int(), first_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', last_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListWeaknessesResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetWeaknessResponseMock = (overrideResponse: Partial<Extract<Weakness, object>> = {}): Weakness => ({id: faker.string.uuid(), course_id: faker.string.uuid(), lesson_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), concept_key: faker.string.alpha({length: {min: 10, max: 20}}), concept_label: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), evidence: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({})), severity: faker.number.int({min: 1, max: 5}), review_count: faker.number.int({min: 0}), mastered: faker.datatype.boolean(), version: faker.number.int(), first_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', last_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetWeaknessResponseMock200 = (overrideResponse: Partial<Extract<Weakness, object>> = {}): Weakness => ({id: faker.string.uuid(), course_id: faker.string.uuid(), lesson_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), concept_key: faker.string.alpha({length: {min: 10, max: 20}}), concept_label: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), evidence: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({})), severity: faker.number.int({min: 1, max: 5}), review_count: faker.number.int({min: 0}), mastered: faker.datatype.boolean(), version: faker.number.int(), first_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', last_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetWeaknessResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getResolveWeaknessResponseMock = (overrideResponse: Partial<Extract<Weakness, object>> = {}): Weakness => ({id: faker.string.uuid(), course_id: faker.string.uuid(), lesson_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), concept_key: faker.string.alpha({length: {min: 10, max: 20}}), concept_label: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), evidence: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({})), severity: faker.number.int({min: 1, max: 5}), review_count: faker.number.int({min: 0}), mastered: faker.datatype.boolean(), version: faker.number.int(), first_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', last_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getResolveWeaknessResponseMock200 = (overrideResponse: Partial<Extract<Weakness, object>> = {}): Weakness => ({id: faker.string.uuid(), course_id: faker.string.uuid(), lesson_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), concept_key: faker.string.alpha({length: {min: 10, max: 20}}), concept_label: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), evidence: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({})), severity: faker.number.int({min: 1, max: 5}), review_count: faker.number.int({min: 0}), mastered: faker.datatype.boolean(), version: faker.number.int(), first_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', last_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getResolveWeaknessResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getReopenWeaknessResponseMock = (overrideResponse: Partial<Extract<Weakness, object>> = {}): Weakness => ({id: faker.string.uuid(), course_id: faker.string.uuid(), lesson_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), concept_key: faker.string.alpha({length: {min: 10, max: 20}}), concept_label: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), evidence: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({})), severity: faker.number.int({min: 1, max: 5}), review_count: faker.number.int({min: 0}), mastered: faker.datatype.boolean(), version: faker.number.int(), first_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', last_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getReopenWeaknessResponseMock200 = (overrideResponse: Partial<Extract<Weakness, object>> = {}): Weakness => ({id: faker.string.uuid(), course_id: faker.string.uuid(), lesson_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), concept_key: faker.string.alpha({length: {min: 10, max: 20}}), concept_label: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), evidence: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({})), severity: faker.number.int({min: 1, max: 5}), review_count: faker.number.int({min: 0}), mastered: faker.datatype.boolean(), version: faker.number.int(), first_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', last_seen_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getReopenWeaknessResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getListWeaknessesMockHandler = (overrideResponse?: WeaknessList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<WeaknessList> | WeaknessList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/weaknesses', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListWeaknessesResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListWeaknessesMockHandler200 = (overrideResponse?: WeaknessList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<WeaknessList> | WeaknessList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/weaknesses', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListWeaknessesResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListWeaknessesMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/weaknesses', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListWeaknessesResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetWeaknessMockHandler = (overrideResponse?: Weakness | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Weakness> | Weakness), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/weaknesses/:weaknessId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetWeaknessResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetWeaknessMockHandler200 = (overrideResponse?: Weakness | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Weakness> | Weakness), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/weaknesses/:weaknessId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetWeaknessResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetWeaknessMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/weaknesses/:weaknessId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetWeaknessResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getResolveWeaknessMockHandler = (overrideResponse?: Weakness | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Weakness> | Weakness), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/weaknesses/:weaknessId/resolve', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getResolveWeaknessResponseMock(),
      { status: 200
      })
  }, options)
}


export const getResolveWeaknessMockHandler200 = (overrideResponse?: Weakness | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Weakness> | Weakness), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/weaknesses/:weaknessId/resolve', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getResolveWeaknessResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getResolveWeaknessMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/weaknesses/:weaknessId/resolve', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getResolveWeaknessResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getReopenWeaknessMockHandler = (overrideResponse?: Weakness | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Weakness> | Weakness), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/weaknesses/:weaknessId/reopen', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getReopenWeaknessResponseMock(),
      { status: 200
      })
  }, options)
}


export const getReopenWeaknessMockHandler200 = (overrideResponse?: Weakness | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Weakness> | Weakness), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/weaknesses/:weaknessId/reopen', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getReopenWeaknessResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getReopenWeaknessMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/weaknesses/:weaknessId/reopen', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getReopenWeaknessResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getWeaknessesMock = () => [
  getListWeaknessesMockHandler(),
  getGetWeaknessMockHandler(),
  getResolveWeaknessMockHandler(),
  getReopenWeaknessMockHandler()
]
