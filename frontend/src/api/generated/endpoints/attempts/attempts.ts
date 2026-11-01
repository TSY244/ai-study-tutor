/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  AnswerSave,
  AsyncAcceptedResponse,
  Attempt,
  AttemptList,
  AttemptSubmitAccepted,
  AttemptSubmitCompleted,
  ListLessonAttemptsParams,
  SavedAnswer,
  SubmitAttemptBody
} from '../../model';

import { apiFetch } from '../../../http';

export const getListLessonAttemptsUrl = (lessonId: string,
    params?: ListLessonAttemptsParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/lessons/${lessonId}/attempts?${stringifiedParams}` : `/api/v1/lessons/${lessonId}/attempts`
}

/**
 * 按 started_at 倒序（最新在前）返回该课时全部试卷的作答摘要，游标分页。
 * @summary 查询课时作答历史
 */
export const listLessonAttempts = async (lessonId: string,
    params?: ListLessonAttemptsParams, options?: RequestInit): Promise<AttemptList> => {

  return apiFetch<AttemptList>(getListLessonAttemptsUrl(lessonId,params),
  {
    ...options,
    method: 'GET'


  }
);}


export const getCreateAttemptUrl = (quizId: string,) => {




  return `/api/v1/quizzes/${quizId}/attempts`
}

/**
 * @summary 开始一次作答
 */
export const createAttempt = async (quizId: string, options?: RequestInit): Promise<Attempt> => {

  return apiFetch<Attempt>(getCreateAttemptUrl(quizId),
  {
    ...options,
    method: 'POST'


  }
);}


export const getGetAttemptUrl = (attemptId: string,) => {




  return `/api/v1/attempts/${attemptId}`
}

/**
 * @summary 查询作答和当前批阅状态
 */
export const getAttempt = async (attemptId: string, options?: RequestInit): Promise<Attempt> => {

  return apiFetch<Attempt>(getGetAttemptUrl(attemptId),
  {
    ...options,
    method: 'GET'


  }
);}


export const getDeleteAttemptUrl = (attemptId: string,) => {




  return `/api/v1/attempts/${attemptId}`
}

/**
 * 删除答案和该作答产生的掌握度快照，清理薄弱点中的该作答证据，并根据剩余成绩重新计算当前进度；人工维护的薄弱点状态保留。有活动批阅/生成任务时返回 409。
 * @summary 删除作答记录
 */
export const deleteAttempt = async (attemptId: string, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteAttemptUrl(attemptId),
  {
    ...options,
    method: 'DELETE'


  }
);}


export const getSaveAttemptAnswerUrl = (attemptId: string,
    questionId: string,) => {




  return `/api/v1/attempts/${attemptId}/answers/${questionId}`
}

/**
 * @summary 自动保存一道题的答案
 */
export const saveAttemptAnswer = async (attemptId: string,
    questionId: string,
    answerSave: AnswerSave, options?: RequestInit): Promise<SavedAnswer> => {

  return apiFetch<SavedAnswer>(getSaveAttemptAnswerUrl(attemptId,questionId),
  {
    ...options,
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(answerSave)
  }
);}


export const getSubmitAttemptUrl = (attemptId: string,) => {




  return `/api/v1/attempts/${attemptId}/submit`
}

/**
 * @summary 提交作答并按需创建 AI 批阅任务
 */
export const submitAttempt = async (attemptId: string,
    submitAttemptBody: SubmitAttemptBody, options?: RequestInit): Promise<AttemptSubmitCompleted | AttemptSubmitAccepted> => {

  return apiFetch<AttemptSubmitCompleted | AttemptSubmitAccepted>(getSubmitAttemptUrl(attemptId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(submitAttemptBody)
  }
);}


export const getCreateAttemptGradingJobUrl = (attemptId: string,) => {




  return `/api/v1/attempts/${attemptId}/grading-jobs`
}

/**
 * @summary 为批阅失败的 Attempt 创建新的批阅任务
 */
export const createAttemptGradingJob = async (attemptId: string, options?: RequestInit): Promise<AsyncAcceptedResponse> => {

  return apiFetch<AsyncAcceptedResponse>(getCreateAttemptGradingJobUrl(attemptId),
  {
    ...options,
    method: 'POST'


  }
);}


