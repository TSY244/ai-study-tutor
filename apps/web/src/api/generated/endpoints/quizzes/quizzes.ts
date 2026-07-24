/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  AsyncAcceptedResponse,
  ListQuizzesParams,
  Quiz,
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


