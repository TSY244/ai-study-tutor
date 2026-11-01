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
  LessonStatus
} from '../../model';
import type {
  Lesson,
  LessonList,
  ProblemResponse
} from '../../model';


export const getListLessonsResponseMock = (overrideResponse: Partial<Extract<LessonList, object>> = {}): LessonList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), course_id: faker.string.uuid(), ordinal: faker.number.int(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.helpers.arrayElement(Object.values(LessonStatus)), active_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), documents: {raw_text: faker.datatype.boolean(), student_notes: faker.datatype.boolean(), teacher_outline: faker.datatype.boolean()}, mastery: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), undefined]), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListLessonsResponseMock200 = (overrideResponse: Partial<Extract<LessonList, object>> = {}): LessonList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), course_id: faker.string.uuid(), ordinal: faker.number.int(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.helpers.arrayElement(Object.values(LessonStatus)), active_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), documents: {raw_text: faker.datatype.boolean(), student_notes: faker.datatype.boolean(), teacher_outline: faker.datatype.boolean()}, mastery: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), undefined]), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListLessonsResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateLessonResponseMock = (overrideResponse: Partial<Extract<Lesson, object>> = {}): Lesson => ({id: faker.string.uuid(), course_id: faker.string.uuid(), ordinal: faker.number.int(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.helpers.arrayElement(Object.values(LessonStatus)), active_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), documents: {raw_text: faker.datatype.boolean(), student_notes: faker.datatype.boolean(), teacher_outline: faker.datatype.boolean()}, mastery: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), undefined]), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCreateLessonResponseMock201 = (overrideResponse: Partial<Extract<Lesson, object>> = {}): Lesson => ({id: faker.string.uuid(), course_id: faker.string.uuid(), ordinal: faker.number.int(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.helpers.arrayElement(Object.values(LessonStatus)), active_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), documents: {raw_text: faker.datatype.boolean(), student_notes: faker.datatype.boolean(), teacher_outline: faker.datatype.boolean()}, mastery: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), undefined]), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCreateLessonResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetLessonResponseMock = (overrideResponse: Partial<Extract<Lesson, object>> = {}): Lesson => ({id: faker.string.uuid(), course_id: faker.string.uuid(), ordinal: faker.number.int(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.helpers.arrayElement(Object.values(LessonStatus)), active_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), documents: {raw_text: faker.datatype.boolean(), student_notes: faker.datatype.boolean(), teacher_outline: faker.datatype.boolean()}, mastery: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), undefined]), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetLessonResponseMock200 = (overrideResponse: Partial<Extract<Lesson, object>> = {}): Lesson => ({id: faker.string.uuid(), course_id: faker.string.uuid(), ordinal: faker.number.int(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.helpers.arrayElement(Object.values(LessonStatus)), active_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), documents: {raw_text: faker.datatype.boolean(), student_notes: faker.datatype.boolean(), teacher_outline: faker.datatype.boolean()}, mastery: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), undefined]), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getGetLessonResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getUpdateLessonResponseMock = (overrideResponse: Partial<Extract<Lesson, object>> = {}): Lesson => ({id: faker.string.uuid(), course_id: faker.string.uuid(), ordinal: faker.number.int(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.helpers.arrayElement(Object.values(LessonStatus)), active_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), documents: {raw_text: faker.datatype.boolean(), student_notes: faker.datatype.boolean(), teacher_outline: faker.datatype.boolean()}, mastery: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), undefined]), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getUpdateLessonResponseMock200 = (overrideResponse: Partial<Extract<Lesson, object>> = {}): Lesson => ({id: faker.string.uuid(), course_id: faker.string.uuid(), ordinal: faker.number.int(), title: faker.string.alpha({length: {min: 10, max: 20}}), description: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.helpers.arrayElement(Object.values(LessonStatus)), active_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), documents: {raw_text: faker.datatype.boolean(), student_notes: faker.datatype.boolean(), teacher_outline: faker.datatype.boolean()}, mastery: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({min: 0, max: 1, fractionDigits: 2}), null]), undefined]), version: faker.number.int(), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getUpdateLessonResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})



export const getDeleteLessonResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})



export const getBatchDeleteLessonsResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getListLessonsMockHandler = (overrideResponse?: LessonList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<LessonList> | LessonList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses/:courseId/lessons', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListLessonsResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListLessonsMockHandler200 = (overrideResponse?: LessonList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<LessonList> | LessonList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses/:courseId/lessons', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListLessonsResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListLessonsMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/courses/:courseId/lessons', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListLessonsResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateLessonMockHandler = (overrideResponse?: Lesson | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Lesson> | Lesson), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/courses/:courseId/lessons', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateLessonResponseMock(),
      { status: 201
      })
  }, options)
}


export const getCreateLessonMockHandler201 = (overrideResponse?: Lesson | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Lesson> | Lesson), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/courses/:courseId/lessons', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateLessonResponseMock201(),
      { status: 201
      })
  }, options)
}


export const getCreateLessonMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/courses/:courseId/lessons', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateLessonResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetLessonMockHandler = (overrideResponse?: Lesson | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Lesson> | Lesson), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetLessonResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetLessonMockHandler200 = (overrideResponse?: Lesson | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Lesson> | Lesson), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetLessonResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetLessonMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetLessonResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getUpdateLessonMockHandler = (overrideResponse?: Lesson | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<Lesson> | Lesson), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/lessons/:lessonId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateLessonResponseMock(),
      { status: 200
      })
  }, options)
}


export const getUpdateLessonMockHandler200 = (overrideResponse?: Lesson | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<Lesson> | Lesson), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/lessons/:lessonId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateLessonResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getUpdateLessonMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/lessons/:lessonId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateLessonResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getDeleteLessonMockHandler = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/lessons/:lessonId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteLessonMockHandler204 = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/lessons/:lessonId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteLessonMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/lessons/:lessonId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getDeleteLessonResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getBatchDeleteLessonsMockHandler = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons\\:batch-delete', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getBatchDeleteLessonsMockHandler204 = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons\\:batch-delete', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getBatchDeleteLessonsMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/lessons\\:batch-delete', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getBatchDeleteLessonsResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getLessonsMock = () => [
  getListLessonsMockHandler(),
  getCreateLessonMockHandler(),
  getGetLessonMockHandler(),
  getUpdateLessonMockHandler(),
  getDeleteLessonMockHandler(),
  getBatchDeleteLessonsMockHandler()
]
