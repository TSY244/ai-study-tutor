/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  AsyncAcceptedResponse,
  Job,
  JobEventList,
  JobList,
  ListJobEventsParams,
  ListJobsParams
} from '../../model';

import { apiFetch } from '../../../http';

export const getListJobsUrl = (params?: ListJobsParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/jobs?${stringifiedParams}` : `/api/v1/jobs`
}

/**
 * @summary 查询任务列表
 */
export const listJobs = async (params?: ListJobsParams, options?: RequestInit): Promise<JobList> => {

  return apiFetch<JobList>(getListJobsUrl(params),
  {
    ...options,
    method: 'GET'


  }
);}


export const getGetJobUrl = (jobId: string,) => {




  return `/api/v1/jobs/${jobId}`
}

/**
 * @summary 查询任务详情
 */
export const getJob = async (jobId: string, options?: RequestInit): Promise<Job> => {

  return apiFetch<Job>(getGetJobUrl(jobId),
  {
    ...options,
    method: 'GET'


  }
);}


export const getCancelJobUrl = (jobId: string,) => {




  return `/api/v1/jobs/${jobId}/cancel`
}

/**
 * @summary 请求取消任务
 */
export const cancelJob = async (jobId: string, options?: RequestInit): Promise<Job> => {

  return apiFetch<Job>(getCancelJobUrl(jobId),
  {
    ...options,
    method: 'POST'


  }
);}


export const getRetryJobUrl = (jobId: string,) => {




  return `/api/v1/jobs/${jobId}/retry`
}

/**
 * @summary 重试失败任务
 */
export const retryJob = async (jobId: string, options?: RequestInit): Promise<AsyncAcceptedResponse> => {

  return apiFetch<AsyncAcceptedResponse>(getRetryJobUrl(jobId),
  {
    ...options,
    method: 'POST'


  }
);}


export const getListJobEventsUrl = (jobId: string,
    params?: ListJobEventsParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/jobs/${jobId}/events?${stringifiedParams}` : `/api/v1/jobs/${jobId}/events`
}

/**
 * @summary 查询任务事件历史
 */
export const listJobEvents = async (jobId: string,
    params?: ListJobEventsParams, options?: RequestInit): Promise<JobEventList> => {

  return apiFetch<JobEventList>(getListJobEventsUrl(jobId,params),
  {
    ...options,
    method: 'GET'


  }
);}


