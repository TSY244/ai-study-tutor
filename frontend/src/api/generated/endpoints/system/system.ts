/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  AsyncAcceptedResponse,
  Health,
  SystemStatus
} from '../../model';

import { apiFetch } from '../../../http';

export const getGetHealthUrl = () => {




  return `/healthz`
}

/**
 * @summary 进程存活检查
 */
export const getHealth = async ( options?: RequestInit): Promise<Health> => {

  return apiFetch<Health>(getGetHealthUrl(),
  {
    ...options,
    method: 'GET'


  }
);}


export const getGetReadinessUrl = () => {




  return `/readyz`
}

/**
 * @summary 服务就绪检查
 */
export const getReadiness = async ( options?: RequestInit): Promise<Health> => {

  return apiFetch<Health>(getGetReadinessUrl(),
  {
    ...options,
    method: 'GET'


  }
);}


export const getGetSystemStatusUrl = () => {




  return `/api/v1/system/status`
}

/**
 * @summary 查询非敏感系统状态
 */
export const getSystemStatus = async ( options?: RequestInit): Promise<SystemStatus> => {

  return apiFetch<SystemStatus>(getGetSystemStatusUrl(),
  {
    ...options,
    method: 'GET'


  }
);}


export const getCreateClaudeConnectivityTestUrl = () => {




  return `/api/v1/system/claude-tests`
}

/**
 * @summary 创建 Claude 连通性测试任务
 */
export const createClaudeConnectivityTest = async ( options?: RequestInit): Promise<AsyncAcceptedResponse> => {

  return apiFetch<AsyncAcceptedResponse>(getCreateClaudeConnectivityTestUrl(),
  {
    ...options,
    method: 'POST'


  }
);}


