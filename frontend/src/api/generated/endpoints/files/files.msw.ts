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
  ProblemResponse
} from '../../model';


export const getDownloadFileResponseMock = (): ArrayBuffer => (new ArrayBuffer(faker.number.int({ min: 1, max: 64 })))


export const getDownloadFileResponseMock200 = (): ArrayBuffer => (new ArrayBuffer(faker.number.int({ min: 1, max: 64 })))


export const getDownloadFileResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getDownloadFileMockHandler = (overrideResponse?: ArrayBuffer | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ArrayBuffer> | ArrayBuffer), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/files/:fileId/download', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {

  const binaryBody = overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getDownloadFileResponseMock();
    return HttpResponse.arrayBuffer(
      binaryBody instanceof ArrayBuffer
        ? binaryBody
        : new ArrayBuffer(0),
      { status: 200,
        headers: { 'Content-Type': 'application/octet-stream' }
      })
  }, options)
}


export const getDownloadFileMockHandler200 = (overrideResponse?: ArrayBuffer | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ArrayBuffer> | ArrayBuffer), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/files/:fileId/download', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {

  const binaryBody = overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getDownloadFileResponseMock200();
    return HttpResponse.arrayBuffer(
      binaryBody instanceof ArrayBuffer
        ? binaryBody
        : new ArrayBuffer(0),
      { status: 200,
        headers: { 'Content-Type': 'application/octet-stream' }
      })
  }, options)
}


export const getDownloadFileMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/files/:fileId/download', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getDownloadFileResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getFilesMock = () => [
  getDownloadFileMockHandler()
]
