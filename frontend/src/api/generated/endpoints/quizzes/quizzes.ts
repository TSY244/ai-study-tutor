/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  AsyncAcceptedResponse,
  ListQuizzesParams,
  ManualQuizCreate,
  ManualQuizUpdate,
  Quiz,
  QuizEditor,
  QuizGenerationCreate,
  QuizList,
  ReviewQuizGenerationCreate
} from '../../model';

import { apiFetch } from '../../../http';

export const getCreateQuizGenerationUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/quiz-generations`
}

/**
 * @summary 创建常规试卷生成任务
 */
export const createQuizGeneration = async (lessonId: string,
    quizGenerationCreate: QuizGenerationCreate, options?: RequestInit): Promise<AsyncAcceptedResponse> => {

  return apiFetch<AsyncAcceptedResponse>(getCreateQuizGenerationUrl(lessonId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(quizGenerationCreate)
  }
);}


export const getCreateReviewQuizGenerationUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/review-quiz-generations`
}

/**
 * @summary 创建复习试卷生成任务
 */
export const createReviewQuizGeneration = async (lessonId: string,
    reviewQuizGenerationCreate: ReviewQuizGenerationCreate, options?: RequestInit): Promise<AsyncAcceptedResponse> => {

  return apiFetch<AsyncAcceptedResponse>(getCreateReviewQuizGenerationUrl(lessonId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(reviewQuizGenerationCreate)
  }
);}


export const getListQuizzesUrl = (lessonId: string,
    params?: ListQuizzesParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/lessons/${lessonId}/quizzes?${stringifiedParams}` : `/api/v1/lessons/${lessonId}/quizzes`
}

/**
 * @summary 查询课时试卷
 */
export const listQuizzes = async (lessonId: string,
    params?: ListQuizzesParams, options?: RequestInit): Promise<QuizList> => {

  return apiFetch<QuizList>(getListQuizzesUrl(lessonId,params),
  {
    ...options,
    method: 'GET'


  }
);}


export const getCreateManualQuizUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/quizzes`
}

/**
 * @summary 手工创建测验与题目
 */
export const createManualQuiz = async (lessonId: string,
    manualQuizCreate: ManualQuizCreate, options?: RequestInit): Promise<QuizEditor> => {

  return apiFetch<QuizEditor>(getCreateManualQuizUrl(lessonId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(manualQuizCreate)
  }
);}


export const getGetQuizUrl = (quizId: string,) => {




  return `/api/v1/quizzes/${quizId}`
}

/**
 * @summary 查询学生可见的试卷
 */
export const getQuiz = async (quizId: string, options?: RequestInit): Promise<Quiz> => {

  return apiFetch<Quiz>(getGetQuizUrl(quizId),
  {
    ...options,
    method: 'GET'


  }
);}


export const getUpdateQuizUrl = (quizId: string,) => {




  return `/api/v1/quizzes/${quizId}`
}

/**
 * 携带当前 version。只修改标题/备注时保留 ID；提供完整 questions 数组时创建新的试卷 ID，旧试卷归档并从测验列表移除，历史作答继续引用原题目。调用方必须采用响应中的新 ID 和 version。
 * @summary 编辑测验及题目
 */
export const updateQuiz = async (quizId: string,
    manualQuizUpdate: ManualQuizUpdate, options?: RequestInit): Promise<QuizEditor> => {

  return apiFetch<QuizEditor>(getUpdateQuizUrl(quizId),
  {
    ...options,
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(manualQuizUpdate)
  }
);}


export const getDeleteQuizUrl = (quizId: string,) => {




  return `/api/v1/quizzes/${quizId}`
}

/**
 * 级联删除测验、作答、答案和相关进度快照，清理薄弱点证据并重新计算进度；有进行中任务时返回 409 resource_busy。历史修改前试卷 ID 也可用此接口清理。
 * @summary 删除测验
 */
export const deleteQuiz = async (quizId: string, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteQuizUrl(quizId),
  {
    ...options,
    method: 'DELETE'


  }
);}


export const getGetQuizEditorUrl = (quizId: string,) => {




  return `/api/v1/quizzes/${quizId}/editor`
}

/**
 * 内容管理专用表示，含标准答案与解析。学生 GET /quizzes/{quiz_id} 的答案隐藏规则不变。
 * @summary 读取测验编辑内容（含参考答案）
 */
export const getQuizEditor = async (quizId: string, options?: RequestInit): Promise<QuizEditor> => {

  return apiFetch<QuizEditor>(getGetQuizEditorUrl(quizId),
  {
    ...options,
    method: 'GET'


  }
);}


