/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  CreateSummaryEvaluationBody,
  SummaryEvaluation,
  SummaryEvaluationAccepted
} from '../../model';

import { apiFetch } from '../../../http';

export const getCreateSummaryEvaluationUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/summary-evaluations`
}

/**
 * @summary 创建学习总结评估任务
 */
export const createSummaryEvaluation = async (lessonId: string,
    createSummaryEvaluationBody: CreateSummaryEvaluationBody, options?: RequestInit): Promise<SummaryEvaluationAccepted> => {

  return apiFetch<SummaryEvaluationAccepted>(getCreateSummaryEvaluationUrl(lessonId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(createSummaryEvaluationBody)
  }
);}


export const getGetSummaryEvaluationUrl = (evaluationId: string,) => {




  return `/api/v1/summary-evaluations/${evaluationId}`
}

/**
 * @summary 查询学习总结评估
 */
export const getSummaryEvaluation = async (evaluationId: string, options?: RequestInit): Promise<SummaryEvaluation> => {

  return apiFetch<SummaryEvaluation>(getGetSummaryEvaluationUrl(evaluationId),
  {
    ...options,
    method: 'GET'


  }
);}


