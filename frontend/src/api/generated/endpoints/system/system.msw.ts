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
  GatewayProtocol,
  JobKind,
  JobStatus
} from '../../model';
import type {
  AsyncAcceptedResponse,
  BilibiliCookieStatus,
  BilibiliLoginStatus,
  ClaudeCodeSettingsSaved,
  ClaudeKey,
  ClaudeKeyList,
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

export const getListClaudeKeysResponseMock = (overrideResponse: Partial<Extract<ClaudeKeyList, object>> = {}): ClaudeKeyList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({protocol: faker.helpers.arrayElement(Object.values(GatewayProtocol)), id: faker.string.alpha({length: {min: 10, max: 20}}), name: faker.string.alpha({length: {min: 10, max: 20}}), masked_key: faker.string.alpha({length: {min: 10, max: 20}}), fingerprint: faker.string.alpha({length: {min: 10, max: 20}}), model: faker.string.alpha({length: {min: 10, max: 20}}), base_url: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), enabled: faker.datatype.boolean(), priority: faker.number.int({min: 0}), weight: faker.number.int({min: 1}), timeout_seconds: faker.number.int({min: 1, max: 3600}), cooldown_seconds: faker.number.int({min: 1, max: 86400}), max_consecutive_failures: faker.number.int({min: 1, max: 100}), status: faker.helpers.arrayElement(['unknown','healthy','cooling_down','auth_failed','quota_exhausted','disabled','testing'] as const), consecutive_failures: faker.number.int({min: 0}), last_error_code: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), last_success_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_failure_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), cooldown_until: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_latency_ms: faker.helpers.arrayElement([faker.number.int({min: 0}), null]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), routing: faker.helpers.arrayElement(['priority_failover'] as const), legacy_fallback: faker.datatype.boolean(), write_enabled: faker.datatype.boolean(), ...overrideResponse})


export const getListClaudeKeysResponseMock200 = (overrideResponse: Partial<Extract<ClaudeKeyList, object>> = {}): ClaudeKeyList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({protocol: faker.helpers.arrayElement(Object.values(GatewayProtocol)), id: faker.string.alpha({length: {min: 10, max: 20}}), name: faker.string.alpha({length: {min: 10, max: 20}}), masked_key: faker.string.alpha({length: {min: 10, max: 20}}), fingerprint: faker.string.alpha({length: {min: 10, max: 20}}), model: faker.string.alpha({length: {min: 10, max: 20}}), base_url: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), enabled: faker.datatype.boolean(), priority: faker.number.int({min: 0}), weight: faker.number.int({min: 1}), timeout_seconds: faker.number.int({min: 1, max: 3600}), cooldown_seconds: faker.number.int({min: 1, max: 86400}), max_consecutive_failures: faker.number.int({min: 1, max: 100}), status: faker.helpers.arrayElement(['unknown','healthy','cooling_down','auth_failed','quota_exhausted','disabled','testing'] as const), consecutive_failures: faker.number.int({min: 0}), last_error_code: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), last_success_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_failure_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), cooldown_until: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_latency_ms: faker.helpers.arrayElement([faker.number.int({min: 0}), null]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), routing: faker.helpers.arrayElement(['priority_failover'] as const), legacy_fallback: faker.datatype.boolean(), write_enabled: faker.datatype.boolean(), ...overrideResponse})


export const getListClaudeKeysResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateClaudeKeyResponseMock = (overrideResponse: Partial<Extract<ClaudeKey, object>> = {}): ClaudeKey => ({protocol: faker.helpers.arrayElement(Object.values(GatewayProtocol)), id: faker.string.alpha({length: {min: 10, max: 20}}), name: faker.string.alpha({length: {min: 10, max: 20}}), masked_key: faker.string.alpha({length: {min: 10, max: 20}}), fingerprint: faker.string.alpha({length: {min: 10, max: 20}}), model: faker.string.alpha({length: {min: 10, max: 20}}), base_url: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), enabled: faker.datatype.boolean(), priority: faker.number.int({min: 0}), weight: faker.number.int({min: 1}), timeout_seconds: faker.number.int({min: 1, max: 3600}), cooldown_seconds: faker.number.int({min: 1, max: 86400}), max_consecutive_failures: faker.number.int({min: 1, max: 100}), status: faker.helpers.arrayElement(['unknown','healthy','cooling_down','auth_failed','quota_exhausted','disabled','testing'] as const), consecutive_failures: faker.number.int({min: 0}), last_error_code: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), last_success_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_failure_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), cooldown_until: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_latency_ms: faker.helpers.arrayElement([faker.number.int({min: 0}), null]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCreateClaudeKeyResponseMock201 = (overrideResponse: Partial<Extract<ClaudeKey, object>> = {}): ClaudeKey => ({protocol: faker.helpers.arrayElement(Object.values(GatewayProtocol)), id: faker.string.alpha({length: {min: 10, max: 20}}), name: faker.string.alpha({length: {min: 10, max: 20}}), masked_key: faker.string.alpha({length: {min: 10, max: 20}}), fingerprint: faker.string.alpha({length: {min: 10, max: 20}}), model: faker.string.alpha({length: {min: 10, max: 20}}), base_url: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), enabled: faker.datatype.boolean(), priority: faker.number.int({min: 0}), weight: faker.number.int({min: 1}), timeout_seconds: faker.number.int({min: 1, max: 3600}), cooldown_seconds: faker.number.int({min: 1, max: 86400}), max_consecutive_failures: faker.number.int({min: 1, max: 100}), status: faker.helpers.arrayElement(['unknown','healthy','cooling_down','auth_failed','quota_exhausted','disabled','testing'] as const), consecutive_failures: faker.number.int({min: 0}), last_error_code: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), last_success_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_failure_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), cooldown_until: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_latency_ms: faker.helpers.arrayElement([faker.number.int({min: 0}), null]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getCreateClaudeKeyResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getUpdateClaudeKeyResponseMock = (overrideResponse: Partial<Extract<ClaudeKey, object>> = {}): ClaudeKey => ({protocol: faker.helpers.arrayElement(Object.values(GatewayProtocol)), id: faker.string.alpha({length: {min: 10, max: 20}}), name: faker.string.alpha({length: {min: 10, max: 20}}), masked_key: faker.string.alpha({length: {min: 10, max: 20}}), fingerprint: faker.string.alpha({length: {min: 10, max: 20}}), model: faker.string.alpha({length: {min: 10, max: 20}}), base_url: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), enabled: faker.datatype.boolean(), priority: faker.number.int({min: 0}), weight: faker.number.int({min: 1}), timeout_seconds: faker.number.int({min: 1, max: 3600}), cooldown_seconds: faker.number.int({min: 1, max: 86400}), max_consecutive_failures: faker.number.int({min: 1, max: 100}), status: faker.helpers.arrayElement(['unknown','healthy','cooling_down','auth_failed','quota_exhausted','disabled','testing'] as const), consecutive_failures: faker.number.int({min: 0}), last_error_code: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), last_success_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_failure_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), cooldown_until: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_latency_ms: faker.helpers.arrayElement([faker.number.int({min: 0}), null]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getUpdateClaudeKeyResponseMock200 = (overrideResponse: Partial<Extract<ClaudeKey, object>> = {}): ClaudeKey => ({protocol: faker.helpers.arrayElement(Object.values(GatewayProtocol)), id: faker.string.alpha({length: {min: 10, max: 20}}), name: faker.string.alpha({length: {min: 10, max: 20}}), masked_key: faker.string.alpha({length: {min: 10, max: 20}}), fingerprint: faker.string.alpha({length: {min: 10, max: 20}}), model: faker.string.alpha({length: {min: 10, max: 20}}), base_url: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), enabled: faker.datatype.boolean(), priority: faker.number.int({min: 0}), weight: faker.number.int({min: 1}), timeout_seconds: faker.number.int({min: 1, max: 3600}), cooldown_seconds: faker.number.int({min: 1, max: 86400}), max_consecutive_failures: faker.number.int({min: 1, max: 100}), status: faker.helpers.arrayElement(['unknown','healthy','cooling_down','auth_failed','quota_exhausted','disabled','testing'] as const), consecutive_failures: faker.number.int({min: 0}), last_error_code: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), last_success_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_failure_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), cooldown_until: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), last_latency_ms: faker.helpers.arrayElement([faker.number.int({min: 0}), null]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getUpdateClaudeKeyResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})



export const getDeleteClaudeKeyResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateClaudeKeyTestResponseMock = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateClaudeKeyTestResponseMock202 = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateClaudeKeyTestResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getSaveClaudeKeyToClaudeCodeSettingsResponseMock = (overrideResponse: Partial<Extract<ClaudeCodeSettingsSaved, object>> = {}): ClaudeCodeSettingsSaved => ({status: faker.helpers.arrayElement(['saved'] as const), target: faker.helpers.arrayElement(['~/.claude/settings.json'] as const), ...overrideResponse})


export const getSaveClaudeKeyToClaudeCodeSettingsResponseMock200 = (overrideResponse: Partial<Extract<ClaudeCodeSettingsSaved, object>> = {}): ClaudeCodeSettingsSaved => ({status: faker.helpers.arrayElement(['saved'] as const), target: faker.helpers.arrayElement(['~/.claude/settings.json'] as const), ...overrideResponse})


export const getSaveClaudeKeyToClaudeCodeSettingsResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetBilibiliCookieStatusResponseMock = (overrideResponse: Partial<Extract<BilibiliCookieStatus, object>> = {}): BilibiliCookieStatus => ({configured: faker.datatype.boolean(), updated_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), sessdata_expires_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), expired: faker.datatype.boolean(), masked_sessdata: faker.string.alpha({length: {min: 10, max: 20}}), cookie_count: faker.number.int({min: 0}), ...overrideResponse})


export const getGetBilibiliCookieStatusResponseMock200 = (overrideResponse: Partial<Extract<BilibiliCookieStatus, object>> = {}): BilibiliCookieStatus => ({configured: faker.datatype.boolean(), updated_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), sessdata_expires_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), expired: faker.datatype.boolean(), masked_sessdata: faker.string.alpha({length: {min: 10, max: 20}}), cookie_count: faker.number.int({min: 0}), ...overrideResponse})


export const getGetBilibiliCookieStatusResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getSaveBilibiliCookieResponseMock = (overrideResponse: Partial<Extract<BilibiliCookieStatus, object>> = {}): BilibiliCookieStatus => ({configured: faker.datatype.boolean(), updated_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), sessdata_expires_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), expired: faker.datatype.boolean(), masked_sessdata: faker.string.alpha({length: {min: 10, max: 20}}), cookie_count: faker.number.int({min: 0}), ...overrideResponse})


export const getSaveBilibiliCookieResponseMock200 = (overrideResponse: Partial<Extract<BilibiliCookieStatus, object>> = {}): BilibiliCookieStatus => ({configured: faker.datatype.boolean(), updated_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), sessdata_expires_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), expired: faker.datatype.boolean(), masked_sessdata: faker.string.alpha({length: {min: 10, max: 20}}), cookie_count: faker.number.int({min: 0}), ...overrideResponse})


export const getSaveBilibiliCookieResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})



export const getDeleteBilibiliCookieResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateBilibiliCookieTestResponseMock = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateBilibiliCookieTestResponseMock202 = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateBilibiliCookieTestResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateBilibiliLoginResponseMock = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateBilibiliLoginResponseMock202 = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateBilibiliLoginResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getPollBilibiliLoginResponseMock = (overrideResponse: Partial<Extract<BilibiliLoginStatus, object>> = {}): BilibiliLoginStatus => ({state: faker.helpers.arrayElement(['generating','waiting_scan','scanned','confirmed','logged_in','expired','failed','cancelled'] as const), message: faker.string.alpha({length: {min: 10, max: 20}}), expires_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', undefined]), qr_image_data_url: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), cookie: faker.helpers.arrayElement([{configured: faker.datatype.boolean(), updated_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), sessdata_expires_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), expired: faker.datatype.boolean(), masked_sessdata: faker.string.alpha({length: {min: 10, max: 20}}), cookie_count: faker.number.int({min: 0})}, undefined]), ...overrideResponse})


export const getPollBilibiliLoginResponseMock200 = (overrideResponse: Partial<Extract<BilibiliLoginStatus, object>> = {}): BilibiliLoginStatus => ({state: faker.helpers.arrayElement(['generating','waiting_scan','scanned','confirmed','logged_in','expired','failed','cancelled'] as const), message: faker.string.alpha({length: {min: 10, max: 20}}), expires_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', undefined]), qr_image_data_url: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), cookie: faker.helpers.arrayElement([{configured: faker.datatype.boolean(), updated_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), sessdata_expires_at: faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), expired: faker.datatype.boolean(), masked_sessdata: faker.string.alpha({length: {min: 10, max: 20}}), cookie_count: faker.number.int({min: 0})}, undefined]), ...overrideResponse})


export const getPollBilibiliLoginResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


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

export const getListClaudeKeysMockHandler = (overrideResponse?: ClaudeKeyList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ClaudeKeyList> | ClaudeKeyList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/system/claude-keys', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListClaudeKeysResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListClaudeKeysMockHandler200 = (overrideResponse?: ClaudeKeyList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ClaudeKeyList> | ClaudeKeyList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/system/claude-keys', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListClaudeKeysResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListClaudeKeysMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/system/claude-keys', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListClaudeKeysResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateClaudeKeyMockHandler = (overrideResponse?: ClaudeKey | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ClaudeKey> | ClaudeKey), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-keys', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateClaudeKeyResponseMock(),
      { status: 201
      })
  }, options)
}


export const getCreateClaudeKeyMockHandler201 = (overrideResponse?: ClaudeKey | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ClaudeKey> | ClaudeKey), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-keys', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateClaudeKeyResponseMock201(),
      { status: 201
      })
  }, options)
}


export const getCreateClaudeKeyMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-keys', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateClaudeKeyResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getUpdateClaudeKeyMockHandler = (overrideResponse?: ClaudeKey | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<ClaudeKey> | ClaudeKey), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/system/claude-keys/:keyId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateClaudeKeyResponseMock(),
      { status: 200
      })
  }, options)
}


export const getUpdateClaudeKeyMockHandler200 = (overrideResponse?: ClaudeKey | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<ClaudeKey> | ClaudeKey), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/system/claude-keys/:keyId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateClaudeKeyResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getUpdateClaudeKeyMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/system/claude-keys/:keyId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getUpdateClaudeKeyResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getDeleteClaudeKeyMockHandler = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/system/claude-keys/:keyId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteClaudeKeyMockHandler204 = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/system/claude-keys/:keyId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteClaudeKeyMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/system/claude-keys/:keyId', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getDeleteClaudeKeyResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateClaudeKeyTestMockHandler = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-keys/:keyId/test', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateClaudeKeyTestResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateClaudeKeyTestMockHandler202 = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-keys/:keyId/test', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateClaudeKeyTestResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateClaudeKeyTestMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-keys/:keyId/test', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateClaudeKeyTestResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getSaveClaudeKeyToClaudeCodeSettingsMockHandler = (overrideResponse?: ClaudeCodeSettingsSaved | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ClaudeCodeSettingsSaved> | ClaudeCodeSettingsSaved), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-keys/:keyId/claude-code-settings', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSaveClaudeKeyToClaudeCodeSettingsResponseMock(),
      { status: 200
      })
  }, options)
}


export const getSaveClaudeKeyToClaudeCodeSettingsMockHandler200 = (overrideResponse?: ClaudeCodeSettingsSaved | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ClaudeCodeSettingsSaved> | ClaudeCodeSettingsSaved), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-keys/:keyId/claude-code-settings', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSaveClaudeKeyToClaudeCodeSettingsResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getSaveClaudeKeyToClaudeCodeSettingsMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/claude-keys/:keyId/claude-code-settings', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSaveClaudeKeyToClaudeCodeSettingsResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetBilibiliCookieStatusMockHandler = (overrideResponse?: BilibiliCookieStatus | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<BilibiliCookieStatus> | BilibiliCookieStatus), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/system/bilibili-cookie', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetBilibiliCookieStatusResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetBilibiliCookieStatusMockHandler200 = (overrideResponse?: BilibiliCookieStatus | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<BilibiliCookieStatus> | BilibiliCookieStatus), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/system/bilibili-cookie', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetBilibiliCookieStatusResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetBilibiliCookieStatusMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/system/bilibili-cookie', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetBilibiliCookieStatusResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getSaveBilibiliCookieMockHandler = (overrideResponse?: BilibiliCookieStatus | ((info: Parameters<Parameters<typeof http.put>[1]>[0]) => Promise<BilibiliCookieStatus> | BilibiliCookieStatus), options?: RequestHandlerOptions) => {
  return http.put('*/api/v1/system/bilibili-cookie', async (info: Parameters<Parameters<typeof http.put>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSaveBilibiliCookieResponseMock(),
      { status: 200
      })
  }, options)
}


export const getSaveBilibiliCookieMockHandler200 = (overrideResponse?: BilibiliCookieStatus | ((info: Parameters<Parameters<typeof http.put>[1]>[0]) => Promise<BilibiliCookieStatus> | BilibiliCookieStatus), options?: RequestHandlerOptions) => {
  return http.put('*/api/v1/system/bilibili-cookie', async (info: Parameters<Parameters<typeof http.put>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSaveBilibiliCookieResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getSaveBilibiliCookieMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.put>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.put('*/api/v1/system/bilibili-cookie', async (info: Parameters<Parameters<typeof http.put>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSaveBilibiliCookieResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getDeleteBilibiliCookieMockHandler = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/system/bilibili-cookie', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteBilibiliCookieMockHandler204 = (overrideResponse?: void | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<void> | void), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/system/bilibili-cookie', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {
  if (typeof overrideResponse === 'function') {await overrideResponse(info); }

    return new HttpResponse(null,
      { status: 204
      })
  }, options)
}


export const getDeleteBilibiliCookieMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.delete>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.delete('*/api/v1/system/bilibili-cookie', async (info: Parameters<Parameters<typeof http.delete>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getDeleteBilibiliCookieResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateBilibiliCookieTestMockHandler = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/bilibili-cookie/tests', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateBilibiliCookieTestResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateBilibiliCookieTestMockHandler202 = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/bilibili-cookie/tests', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateBilibiliCookieTestResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateBilibiliCookieTestMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/bilibili-cookie/tests', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateBilibiliCookieTestResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateBilibiliLoginMockHandler = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/bilibili-cookie/logins', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateBilibiliLoginResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateBilibiliLoginMockHandler202 = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/bilibili-cookie/logins', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateBilibiliLoginResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateBilibiliLoginMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/bilibili-cookie/logins', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateBilibiliLoginResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getPollBilibiliLoginMockHandler = (overrideResponse?: BilibiliLoginStatus | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<BilibiliLoginStatus> | BilibiliLoginStatus), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/bilibili-cookie/logins/:loginId/poll', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getPollBilibiliLoginResponseMock(),
      { status: 200
      })
  }, options)
}


export const getPollBilibiliLoginMockHandler200 = (overrideResponse?: BilibiliLoginStatus | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<BilibiliLoginStatus> | BilibiliLoginStatus), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/bilibili-cookie/logins/:loginId/poll', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getPollBilibiliLoginResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getPollBilibiliLoginMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/system/bilibili-cookie/logins/:loginId/poll', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getPollBilibiliLoginResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getSystemMock = () => [
  getGetHealthMockHandler(),
  getGetReadinessMockHandler(),
  getGetSystemStatusMockHandler(),
  getCreateClaudeConnectivityTestMockHandler(),
  getListClaudeKeysMockHandler(),
  getCreateClaudeKeyMockHandler(),
  getUpdateClaudeKeyMockHandler(),
  getDeleteClaudeKeyMockHandler(),
  getCreateClaudeKeyTestMockHandler(),
  getSaveClaudeKeyToClaudeCodeSettingsMockHandler(),
  getGetBilibiliCookieStatusMockHandler(),
  getSaveBilibiliCookieMockHandler(),
  getDeleteBilibiliCookieMockHandler(),
  getCreateBilibiliCookieTestMockHandler(),
  getCreateBilibiliLoginMockHandler(),
  getPollBilibiliLoginMockHandler()
]
