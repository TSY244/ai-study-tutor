/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  CreateSummaryEvaluationBody,
  ListSummaryEvaluationsParams,
  SummaryEvaluation,
  SummaryEvaluationAccepted,
  SummaryEvaluationCreate,
  SummaryEvaluationList
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


export const getListSummaryEvaluationsUrl = (lessonId: string,
    params?: ListSummaryEvaluationsParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/lessons/${lessonId}/summary-evaluations?${stringifiedParams}` : `/api/v1/lessons/${lessonId}/summary-evaluations`
}

/**
 * @summary 查询课时学习总结历史
 */
export const listSummaryEvaluations = async (lessonId: string,
    params?: ListSummaryEvaluationsParams, options?: RequestInit): Promise<SummaryEvaluationList> => {

  return apiFetch<SummaryEvaluationList>(getListSummaryEvaluationsUrl(lessonId,params),
  {
    ...options,
    method: 'GET'


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


export const getUpdateSummaryEvaluationUrl = (evaluationId: string,) => {




  return `/api/v1/summary-evaluations/${evaluationId}`
}

/**
 * 保留旧总结及其评估结果，返回新 evaluation.id 和评估任务；有活动任务时返回 409。
 * @summary 修改总结并创建新版评估
 */
export const updateSummaryEvaluation = async (evaluationId: string,
    summaryEvaluationCreate: SummaryEvaluationCreate, options?: RequestInit): Promise<SummaryEvaluationAccepted> => {

  return apiFetch<SummaryEvaluationAccepted>(getUpdateSummaryEvaluationUrl(evaluationId),
  {
    ...options,
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(summaryEvaluationCreate)
  }
);}


export const getDeleteSummaryEvaluationUrl = (evaluationId: string,) => {




  return `/api/v1/summary-evaluations/${evaluationId}`
}

/**
 * @summary 删除学习总结评估
 */
export const deleteSummaryEvaluation = async (evaluationId: string, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteSummaryEvaluationUrl(evaluationId),
  {
    ...options,
    method: 'DELETE'


  }
);}


