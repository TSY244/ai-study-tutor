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
  AttemptStatus,
  JobKind,
  JobStatus,
  QuestionType
} from '../../model';
import type {
  AsyncAcceptedResponse,
  Attempt,
  AttemptList,
  AttemptSubmitAccepted,
  AttemptSubmitCompleted,
  ProblemResponse,
  SavedAnswer
} from '../../model';


export const getListLessonAttemptsResponseMock = (overrideResponse: Partial<Extract<AttemptList, object>> = {}): AttemptList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), quiz_id: faker.string.uuid(), quiz_title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.helpers.arrayElement(Object.values(AttemptStatus)), auto_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), final_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), max_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), started_at: faker.date.past().toISOString().slice(0, 19) + 'Z', submitted_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), graded_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined])})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListLessonAttemptsResponseMock200 = (overrideResponse: Partial<Extract<AttemptList, object>> = {}): AttemptList => ({items: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), quiz_id: faker.string.uuid(), quiz_title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.helpers.arrayElement(Object.values(AttemptStatus)), auto_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), final_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), max_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), started_at: faker.date.past().toISOString().slice(0, 19) + 'Z', submitted_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), graded_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined])})), page_info: {next_cursor: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), has_more: faker.datatype.boolean()}, ...overrideResponse})


export const getListLessonAttemptsResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateAttemptResponseMock = (overrideResponse: Partial<Extract<Attempt, object>> = {}): Attempt => ({id: faker.string.uuid(), quiz_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(AttemptStatus)), version: faker.number.int(), auto_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), final_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), max_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), grading_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), grading_error_code: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), grading_error_detail: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), questions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), type: faker.helpers.arrayElement(Object.values(QuestionType)), stem: faker.string.alpha({length: {min: 10, max: 20}}), options: faker.helpers.arrayElement([faker.helpers.arrayElement([{
        [faker.string.alphanumeric(5)]: faker.string.alpha({length: {min: 10, max: 20}})
      },null,]), null]), tested_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), difficulty: faker.helpers.arrayElement(['easy','medium','hard','challenge'] as const), cognitive_level: faker.helpers.arrayElement(['recall','explain','apply','transfer','reflect'] as const), score: faker.number.float({min: 0, fractionDigits: 2})})), answers: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({question_id: faker.string.uuid(), answer: {value: {}}, answer_version: faker.number.int({min: 1}), awarded_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), undefined]), feedback: faker.helpers.arrayElement([faker.helpers.arrayElement([{question_id: faker.string.uuid(), awarded_score: faker.number.float({min: 0, fractionDigits: 2}), max_score: faker.number.float({min: 0, fractionDigits: 2}), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), null]), feedback: faker.string.alpha({length: {min: 10, max: 20}}), correct_understanding: faker.string.alpha({length: {min: 10, max: 20}}), correct_answer: faker.helpers.arrayElement([{value: {}},null,]), explanation: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), weak_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}})))},null,]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), started_at: faker.date.past().toISOString().slice(0, 19) + 'Z', submitted_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), graded_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), ...overrideResponse})


export const getCreateAttemptResponseMock201 = (overrideResponse: Partial<Extract<Attempt, object>> = {}): Attempt => ({id: faker.string.uuid(), quiz_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(AttemptStatus)), version: faker.number.int(), auto_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), final_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), max_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), grading_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), grading_error_code: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), grading_error_detail: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), questions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), type: faker.helpers.arrayElement(Object.values(QuestionType)), stem: faker.string.alpha({length: {min: 10, max: 20}}), options: faker.helpers.arrayElement([faker.helpers.arrayElement([{
        [faker.string.alphanumeric(5)]: faker.string.alpha({length: {min: 10, max: 20}})
      },null,]), null]), tested_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), difficulty: faker.helpers.arrayElement(['easy','medium','hard','challenge'] as const), cognitive_level: faker.helpers.arrayElement(['recall','explain','apply','transfer','reflect'] as const), score: faker.number.float({min: 0, fractionDigits: 2})})), answers: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({question_id: faker.string.uuid(), answer: {value: {}}, answer_version: faker.number.int({min: 1}), awarded_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), undefined]), feedback: faker.helpers.arrayElement([faker.helpers.arrayElement([{question_id: faker.string.uuid(), awarded_score: faker.number.float({min: 0, fractionDigits: 2}), max_score: faker.number.float({min: 0, fractionDigits: 2}), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), null]), feedback: faker.string.alpha({length: {min: 10, max: 20}}), correct_understanding: faker.string.alpha({length: {min: 10, max: 20}}), correct_answer: faker.helpers.arrayElement([{value: {}},null,]), explanation: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), weak_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}})))},null,]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), started_at: faker.date.past().toISOString().slice(0, 19) + 'Z', submitted_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), graded_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), ...overrideResponse})


export const getCreateAttemptResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getGetAttemptResponseMock = (overrideResponse: Partial<Extract<Attempt, object>> = {}): Attempt => ({id: faker.string.uuid(), quiz_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(AttemptStatus)), version: faker.number.int(), auto_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), final_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), max_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), grading_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), grading_error_code: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), grading_error_detail: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), questions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), type: faker.helpers.arrayElement(Object.values(QuestionType)), stem: faker.string.alpha({length: {min: 10, max: 20}}), options: faker.helpers.arrayElement([faker.helpers.arrayElement([{
        [faker.string.alphanumeric(5)]: faker.string.alpha({length: {min: 10, max: 20}})
      },null,]), null]), tested_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), difficulty: faker.helpers.arrayElement(['easy','medium','hard','challenge'] as const), cognitive_level: faker.helpers.arrayElement(['recall','explain','apply','transfer','reflect'] as const), score: faker.number.float({min: 0, fractionDigits: 2})})), answers: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({question_id: faker.string.uuid(), answer: {value: {}}, answer_version: faker.number.int({min: 1}), awarded_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), undefined]), feedback: faker.helpers.arrayElement([faker.helpers.arrayElement([{question_id: faker.string.uuid(), awarded_score: faker.number.float({min: 0, fractionDigits: 2}), max_score: faker.number.float({min: 0, fractionDigits: 2}), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), null]), feedback: faker.string.alpha({length: {min: 10, max: 20}}), correct_understanding: faker.string.alpha({length: {min: 10, max: 20}}), correct_answer: faker.helpers.arrayElement([{value: {}},null,]), explanation: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), weak_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}})))},null,]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), started_at: faker.date.past().toISOString().slice(0, 19) + 'Z', submitted_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), graded_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), ...overrideResponse})


export const getGetAttemptResponseMock200 = (overrideResponse: Partial<Extract<Attempt, object>> = {}): Attempt => ({id: faker.string.uuid(), quiz_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(AttemptStatus)), version: faker.number.int(), auto_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), final_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), max_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), grading_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), grading_error_code: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), grading_error_detail: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), questions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), type: faker.helpers.arrayElement(Object.values(QuestionType)), stem: faker.string.alpha({length: {min: 10, max: 20}}), options: faker.helpers.arrayElement([faker.helpers.arrayElement([{
        [faker.string.alphanumeric(5)]: faker.string.alpha({length: {min: 10, max: 20}})
      },null,]), null]), tested_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), difficulty: faker.helpers.arrayElement(['easy','medium','hard','challenge'] as const), cognitive_level: faker.helpers.arrayElement(['recall','explain','apply','transfer','reflect'] as const), score: faker.number.float({min: 0, fractionDigits: 2})})), answers: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({question_id: faker.string.uuid(), answer: {value: {}}, answer_version: faker.number.int({min: 1}), awarded_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), undefined]), feedback: faker.helpers.arrayElement([faker.helpers.arrayElement([{question_id: faker.string.uuid(), awarded_score: faker.number.float({min: 0, fractionDigits: 2}), max_score: faker.number.float({min: 0, fractionDigits: 2}), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), null]), feedback: faker.string.alpha({length: {min: 10, max: 20}}), correct_understanding: faker.string.alpha({length: {min: 10, max: 20}}), correct_answer: faker.helpers.arrayElement([{value: {}},null,]), explanation: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), weak_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}})))},null,]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), started_at: faker.date.past().toISOString().slice(0, 19) + 'Z', submitted_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), graded_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), ...overrideResponse})


export const getGetAttemptResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getSaveAttemptAnswerResponseMock = (overrideResponse: Partial<Extract<SavedAnswer, object>> = {}): SavedAnswer => ({attempt_id: faker.string.uuid(), question_id: faker.string.uuid(), answer: {value: {}}, answer_version: faker.number.int({min: 1}), attempt_version: faker.number.int({min: 1}), saved_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getSaveAttemptAnswerResponseMock200 = (overrideResponse: Partial<Extract<SavedAnswer, object>> = {}): SavedAnswer => ({attempt_id: faker.string.uuid(), question_id: faker.string.uuid(), answer: {value: {}}, answer_version: faker.number.int({min: 1}), attempt_version: faker.number.int({min: 1}), saved_at: faker.date.past().toISOString().slice(0, 19) + 'Z', ...overrideResponse})


export const getSaveAttemptAnswerResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getSubmitAttemptResponseMock = (overrideResponse: Partial<Extract<AttemptSubmitCompleted | AttemptSubmitAccepted, object>> = {}): AttemptSubmitCompleted | AttemptSubmitAccepted => (faker.helpers.arrayElement([{attempt: {id: faker.string.uuid(), quiz_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(AttemptStatus)), version: faker.number.int(), auto_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), final_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), max_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), grading_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), grading_error_code: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), grading_error_detail: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), questions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), type: faker.helpers.arrayElement(Object.values(QuestionType)), stem: faker.string.alpha({length: {min: 10, max: 20}}), options: faker.helpers.arrayElement([faker.helpers.arrayElement([{
        [faker.string.alphanumeric(5)]: faker.string.alpha({length: {min: 10, max: 20}})
      },null,]), null]), tested_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), difficulty: faker.helpers.arrayElement(['easy','medium','hard','challenge'] as const), cognitive_level: faker.helpers.arrayElement(['recall','explain','apply','transfer','reflect'] as const), score: faker.number.float({min: 0, fractionDigits: 2})})), answers: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({question_id: faker.string.uuid(), answer: {value: {}}, answer_version: faker.number.int({min: 1}), awarded_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), undefined]), feedback: faker.helpers.arrayElement([faker.helpers.arrayElement([{question_id: faker.string.uuid(), awarded_score: faker.number.float({min: 0, fractionDigits: 2}), max_score: faker.number.float({min: 0, fractionDigits: 2}), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), null]), feedback: faker.string.alpha({length: {min: 10, max: 20}}), correct_understanding: faker.string.alpha({length: {min: 10, max: 20}}), correct_answer: faker.helpers.arrayElement([{value: {}},null,]), explanation: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), weak_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}})))},null,]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), started_at: faker.date.past().toISOString().slice(0, 19) + 'Z', submitted_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), graded_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined])}, ...overrideResponse}, {attempt: {id: faker.string.uuid(), quiz_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(AttemptStatus)), version: faker.number.int(), auto_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), final_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), max_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), grading_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), grading_error_code: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), grading_error_detail: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), questions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), type: faker.helpers.arrayElement(Object.values(QuestionType)), stem: faker.string.alpha({length: {min: 10, max: 20}}), options: faker.helpers.arrayElement([faker.helpers.arrayElement([{
        [faker.string.alphanumeric(5)]: faker.string.alpha({length: {min: 10, max: 20}})
      },null,]), null]), tested_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), difficulty: faker.helpers.arrayElement(['easy','medium','hard','challenge'] as const), cognitive_level: faker.helpers.arrayElement(['recall','explain','apply','transfer','reflect'] as const), score: faker.number.float({min: 0, fractionDigits: 2})})), answers: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({question_id: faker.string.uuid(), answer: {value: {}}, answer_version: faker.number.int({min: 1}), awarded_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), undefined]), feedback: faker.helpers.arrayElement([faker.helpers.arrayElement([{question_id: faker.string.uuid(), awarded_score: faker.number.float({min: 0, fractionDigits: 2}), max_score: faker.number.float({min: 0, fractionDigits: 2}), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), null]), feedback: faker.string.alpha({length: {min: 10, max: 20}}), correct_understanding: faker.string.alpha({length: {min: 10, max: 20}}), correct_answer: faker.helpers.arrayElement([{value: {}},null,]), explanation: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), weak_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}})))},null,]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), started_at: faker.date.past().toISOString().slice(0, 19) + 'Z', submitted_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), graded_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined])}, job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse}]))


export const getSubmitAttemptResponseMock200 = (overrideResponse: Partial<Extract<AttemptSubmitCompleted, object>> = {}): AttemptSubmitCompleted => ({attempt: {id: faker.string.uuid(), quiz_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(AttemptStatus)), version: faker.number.int(), auto_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), final_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), max_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), grading_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), grading_error_code: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), grading_error_detail: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), questions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), type: faker.helpers.arrayElement(Object.values(QuestionType)), stem: faker.string.alpha({length: {min: 10, max: 20}}), options: faker.helpers.arrayElement([faker.helpers.arrayElement([{
        [faker.string.alphanumeric(5)]: faker.string.alpha({length: {min: 10, max: 20}})
      },null,]), null]), tested_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), difficulty: faker.helpers.arrayElement(['easy','medium','hard','challenge'] as const), cognitive_level: faker.helpers.arrayElement(['recall','explain','apply','transfer','reflect'] as const), score: faker.number.float({min: 0, fractionDigits: 2})})), answers: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({question_id: faker.string.uuid(), answer: {value: {}}, answer_version: faker.number.int({min: 1}), awarded_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), undefined]), feedback: faker.helpers.arrayElement([faker.helpers.arrayElement([{question_id: faker.string.uuid(), awarded_score: faker.number.float({min: 0, fractionDigits: 2}), max_score: faker.number.float({min: 0, fractionDigits: 2}), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), null]), feedback: faker.string.alpha({length: {min: 10, max: 20}}), correct_understanding: faker.string.alpha({length: {min: 10, max: 20}}), correct_answer: faker.helpers.arrayElement([{value: {}},null,]), explanation: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), weak_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}})))},null,]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), started_at: faker.date.past().toISOString().slice(0, 19) + 'Z', submitted_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), graded_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined])}, ...overrideResponse})


export const getSubmitAttemptResponseMock202 = (overrideResponse: Partial<Extract<AttemptSubmitAccepted, object>> = {}): AttemptSubmitAccepted => ({attempt: {id: faker.string.uuid(), quiz_id: faker.string.uuid(), status: faker.helpers.arrayElement(Object.values(AttemptStatus)), version: faker.number.int(), auto_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), final_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), max_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), grading_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), grading_error_code: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), grading_error_detail: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), questions: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({id: faker.string.uuid(), type: faker.helpers.arrayElement(Object.values(QuestionType)), stem: faker.string.alpha({length: {min: 10, max: 20}}), options: faker.helpers.arrayElement([faker.helpers.arrayElement([{
        [faker.string.alphanumeric(5)]: faker.string.alpha({length: {min: 10, max: 20}})
      },null,]), null]), tested_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}}))), difficulty: faker.helpers.arrayElement(['easy','medium','hard','challenge'] as const), cognitive_level: faker.helpers.arrayElement(['recall','explain','apply','transfer','reflect'] as const), score: faker.number.float({min: 0, fractionDigits: 2})})), answers: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({question_id: faker.string.uuid(), answer: {value: {}}, answer_version: faker.number.int({min: 1}), awarded_score: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.number.float({fractionDigits: 2}), null]), undefined]), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), undefined]), feedback: faker.helpers.arrayElement([faker.helpers.arrayElement([{question_id: faker.string.uuid(), awarded_score: faker.number.float({min: 0, fractionDigits: 2}), max_score: faker.number.float({min: 0, fractionDigits: 2}), is_correct: faker.helpers.arrayElement([faker.datatype.boolean(), null]), feedback: faker.string.alpha({length: {min: 10, max: 20}}), correct_understanding: faker.string.alpha({length: {min: 10, max: 20}}), correct_answer: faker.helpers.arrayElement([{value: {}},null,]), explanation: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), weak_concepts: Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => (faker.string.alpha({length: {min: 10, max: 20}})))},null,]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'})), started_at: faker.date.past().toISOString().slice(0, 19) + 'Z', submitted_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), graded_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined])}, job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getSubmitAttemptResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})

export const getCreateAttemptGradingJobResponseMock = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateAttemptGradingJobResponseMock202 = (overrideResponse: Partial<Extract<AsyncAcceptedResponse, object>> = {}): AsyncAcceptedResponse => ({job: {id: faker.string.uuid(), kind: faker.helpers.arrayElement(Object.values(JobKind)), aggregate_type: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), aggregate_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), generation_family: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.helpers.arrayElement(['lesson_generation'] as const), null]), undefined]), retry_of_job_id: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.uuid(), null]), undefined]), status: faker.helpers.arrayElement(Object.values(JobStatus)), stage: faker.string.alpha({length: {min: 10, max: 20}}), progress: faker.number.int({min: 0, max: 100}), message: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), null]), undefined]), attempt_count: faker.number.int({min: 0}), max_attempts: faker.number.int({min: 1}), retryable: faker.datatype.boolean(), cancel_requested: faker.datatype.boolean(), error: faker.helpers.arrayElement([faker.helpers.arrayElement([{code: faker.string.alpha({length: {min: 10, max: 20}}), detail: faker.string.alpha({length: {min: 10, max: 20}}), retryable: faker.datatype.boolean()},null,]), undefined]), created_at: faker.date.past().toISOString().slice(0, 19) + 'Z', started_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), completed_at: faker.helpers.arrayElement([faker.helpers.arrayElement([faker.date.past().toISOString().slice(0, 19) + 'Z', null]), undefined]), updated_at: faker.date.past().toISOString().slice(0, 19) + 'Z'}, ...overrideResponse})


export const getCreateAttemptGradingJobResponseMockDefault = (overrideResponse: Partial<Extract<ProblemResponse, object>> = {}): ProblemResponse => ({type: faker.string.alpha({length: {min: 10, max: 20}}), title: faker.string.alpha({length: {min: 10, max: 20}}), status: faker.number.int({min: 400, max: 599}), detail: faker.string.alpha({length: {min: 10, max: 20}}), instance: faker.helpers.arrayElement([faker.string.alpha({length: {min: 10, max: 20}}), undefined]), code: faker.string.alpha({length: {min: 10, max: 20}}), request_id: faker.string.alpha({length: {min: 10, max: 20}}), errors: faker.helpers.arrayElement([Array.from({ length: faker.number.int({min: 1, max: 10}) }, (_, i) => i + 1).map(() => ({field: faker.string.alpha({length: {min: 10, max: 20}}), code: faker.string.alpha({length: {min: 10, max: 20}}), message: faker.string.alpha({length: {min: 10, max: 20}})})), undefined]), ...overrideResponse})


export const getListLessonAttemptsMockHandler = (overrideResponse?: AttemptList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<AttemptList> | AttemptList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/attempts', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListLessonAttemptsResponseMock(),
      { status: 200
      })
  }, options)
}


export const getListLessonAttemptsMockHandler200 = (overrideResponse?: AttemptList | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<AttemptList> | AttemptList), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/attempts', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListLessonAttemptsResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getListLessonAttemptsMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/lessons/:lessonId/attempts', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getListLessonAttemptsResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateAttemptMockHandler = (overrideResponse?: Attempt | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Attempt> | Attempt), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/quizzes/:quizId/attempts', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateAttemptResponseMock(),
      { status: 201
      })
  }, options)
}


export const getCreateAttemptMockHandler201 = (overrideResponse?: Attempt | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<Attempt> | Attempt), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/quizzes/:quizId/attempts', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateAttemptResponseMock201(),
      { status: 201
      })
  }, options)
}


export const getCreateAttemptMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/quizzes/:quizId/attempts', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateAttemptResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getGetAttemptMockHandler = (overrideResponse?: Attempt | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Attempt> | Attempt), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/attempts/:attemptId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetAttemptResponseMock(),
      { status: 200
      })
  }, options)
}


export const getGetAttemptMockHandler200 = (overrideResponse?: Attempt | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<Attempt> | Attempt), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/attempts/:attemptId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetAttemptResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getGetAttemptMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.get>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.get('*/api/v1/attempts/:attemptId', async (info: Parameters<Parameters<typeof http.get>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getGetAttemptResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getSaveAttemptAnswerMockHandler = (overrideResponse?: SavedAnswer | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<SavedAnswer> | SavedAnswer), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/attempts/:attemptId/answers/:questionId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSaveAttemptAnswerResponseMock(),
      { status: 200
      })
  }, options)
}


export const getSaveAttemptAnswerMockHandler200 = (overrideResponse?: SavedAnswer | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<SavedAnswer> | SavedAnswer), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/attempts/:attemptId/answers/:questionId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSaveAttemptAnswerResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getSaveAttemptAnswerMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.patch>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.patch('*/api/v1/attempts/:attemptId/answers/:questionId', async (info: Parameters<Parameters<typeof http.patch>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSaveAttemptAnswerResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getSubmitAttemptMockHandler = (overrideResponse?: AttemptSubmitCompleted | AttemptSubmitAccepted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AttemptSubmitCompleted | AttemptSubmitAccepted> | AttemptSubmitCompleted | AttemptSubmitAccepted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/attempts/:attemptId/submit', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSubmitAttemptResponseMock(),
      { status: 200
      })
  }, options)
}


export const getSubmitAttemptMockHandler200 = (overrideResponse?: AttemptSubmitCompleted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AttemptSubmitCompleted> | AttemptSubmitCompleted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/attempts/:attemptId/submit', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSubmitAttemptResponseMock200(),
      { status: 200
      })
  }, options)
}


export const getSubmitAttemptMockHandler202 = (overrideResponse?: AttemptSubmitAccepted | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AttemptSubmitAccepted> | AttemptSubmitAccepted), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/attempts/:attemptId/submit', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSubmitAttemptResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getSubmitAttemptMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/attempts/:attemptId/submit', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getSubmitAttemptResponseMockDefault(),
      { status: 200
      })
  }, options)
}

export const getCreateAttemptGradingJobMockHandler = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/attempts/:attemptId/grading-jobs', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateAttemptGradingJobResponseMock(),
      { status: 202
      })
  }, options)
}


export const getCreateAttemptGradingJobMockHandler202 = (overrideResponse?: AsyncAcceptedResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<AsyncAcceptedResponse> | AsyncAcceptedResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/attempts/:attemptId/grading-jobs', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateAttemptGradingJobResponseMock202(),
      { status: 202
      })
  }, options)
}


export const getCreateAttemptGradingJobMockHandlerDefault = (overrideResponse?: ProblemResponse | ((info: Parameters<Parameters<typeof http.post>[1]>[0]) => Promise<ProblemResponse> | ProblemResponse), options?: RequestHandlerOptions) => {
  return http.post('*/api/v1/attempts/:attemptId/grading-jobs', async (info: Parameters<Parameters<typeof http.post>[1]>[0]) => {


    return HttpResponse.json(overrideResponse !== undefined
    ? (typeof overrideResponse === "function" ? await overrideResponse(info) : overrideResponse)
    : getCreateAttemptGradingJobResponseMockDefault(),
      { status: 200
      })
  }, options)
}
export const getAttemptsMock = () => [
  getListLessonAttemptsMockHandler(),
  getCreateAttemptMockHandler(),
  getGetAttemptMockHandler(),
  getSaveAttemptAnswerMockHandler(),
  getSubmitAttemptMockHandler(),
  getCreateAttemptGradingJobMockHandler()
]
