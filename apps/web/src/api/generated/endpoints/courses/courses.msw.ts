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
  Course,
  CourseList,
  ProblemResponse
} from '../../model';


export const getListCoursesResponseMock = (overrideResponse: Partial<Extract<CourseList, object>> = {}): CourseList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), archived: faker.datatype.boolean(), lesson_count: faker.number.int({min: 0}), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListCoursesResponseMock200 = (overrideResponse: Partial<Extract<CourseList, object>> = {}): CourseList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), archived: faker.datatype.boolean(), lesson_count: faker.number.int({min: 0}), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListCoursesResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateCourseResponseMock = (overrideResponse: Partial<Extract<Course, object>> = {}): Course => ({id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), archived: faker.datatype.boolean(), lesson_count: faker.number.int({min: 0}), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCreateCourseResponseMock201 = (overrideResponse: Partial<Extract<Course, object>> = {}): Course => ({id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), archived: faker.datatype.boolean(), lesson_count: faker.number.int({min: 0}), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCreateCourseResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetCourseResponseMock = (overrideResponse: Partial<Extract<Course, object>> = {}): Course => ({id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), archived: faker.datatype.boolean(), lesson_count: faker.number.int({min: 0}), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetCourseResponseMock200 = (overrideResponse: Partial<Extract<Course, object>> = {}): Course => ({id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), archived: faker.datatype.boolean(), lesson_count: faker.number.int({min: 0}), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetCourseResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getUpdateCourseResponseMock = (overrideResponse: Partial<Extract<Course, object>> = {}): Course => ({id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), archived: faker.datatype.boolean(), lesson_count: faker.number.int({min: 0}), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getUpdateCourseResponseMock200 = (overrideResponse: Partial<Extract<Course, object>> = {}): Course => ({id: faker.string.uuid(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), archived: faker.datatype.boolean(), lesson_count: faker.number.int({min: 0}), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getUpdateCourseResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})



export const getDeleteCourseResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getListCoursesMockHandler = (overrideResponse?: CourseList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<CourseList> | CourseList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListCoursesResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListCoursesMockHandler200 = (overrideResponse?: CourseList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<CourseList> | CourseList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListCoursesResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListCoursesMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListCoursesResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateCourseMockHandler = (overrideResponse?: Course | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Course> | Course), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/courses', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateCourseResponseMock(),
      { status: 201
      })
  }, options)
}


export const getCreateCourseMockHandler201 = (overrideResponse?: Course | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Course> | Course), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/courses', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateCourseResponseMock201(),
      { status: 201
      })
  }, options)
}


export const getCreateCourseMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/courses', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateCourseResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetCourseMockHandler = (overrideResponse?: Course | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Course> | Course), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses/:courseId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetCourseResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetCourseMockHandler200 = (overrideResponse?: Course | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Course> | Course), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses/:courseId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetCourseResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetCourseMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses/:courseId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetCourseResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getUpdateCourseMockHandler = (overrideResponse?: Course | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<Course> | Course), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/courses/:courseId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateCourseResponseMock(),
      { status: 200
      })
  }, options)
}


export const getUpdateCourseMockHandler200 = (overrideResponse?: Course | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<Course> | Course), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/courses/:courseId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateCourseResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getUpdateCourseMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/courses/:courseId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateCourseResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getDeleteCourseMockHandler = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/courses/:courseId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteCourseMockHandler204 = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/courses/:courseId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteCourseMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/courses/:courseId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getDeleteCourseResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getCoursesMock = () => [
  getListCoursesMockHandler(),
  getCreateCourseMockHandler(),
  getGetCourseMockHandler(),
  getUpdateCourseMockHandler(),
  getDeleteCourseMockHandler()
]
