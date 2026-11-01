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
  BackupList,
  ProblemResponse
} from '../../model';


export const getListBackupsResponseMock = (overrideResponse: Partial<Extract<BackupList, object>> = {}): BackupList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), status: faker.helpers.arrayElement(['creating','ready','failed'] as const), application_version: faker.string.alpha({length: {min: 10, max: 20}}), database_version: faker.string.alpha({length: {min: 10, max: 20}}), size_bytes: faker.helpers.arrayElement([faker.number.int({min: 0}), null]), archive_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined])})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListBackupsResponseMock200 = (overrideResponse: Partial<Extract<BackupList, object>> = {}): BackupList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), status: faker.helpers.arrayElement(['creating','ready','failed'] as const), application_version: faker.string.alpha({length: {min: 10, max: 20}}), database_version: faker.string.alpha({length: {min: 10, max: 20}}), size_bytes: faker.helpers.arrayElement([faker.number.int({min: 0}), null]), archive_name: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined])})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListBackupsResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getListBackupsMockHandler = (overrideResponse?: BackupList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<BackupList> | BackupList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/backups', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListBackupsResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListBackupsMockHandler200 = (overrideResponse?: BackupList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<BackupList> | BackupList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/backups', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListBackupsResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListBackupsMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/backups', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListBackupsResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getBackupsMock = () => [
  getListBackupsMockHandler()
]
