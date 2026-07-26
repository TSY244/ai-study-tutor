/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  AsyncAcceptedResponse,
  BilibiliCookieStatus,
  BilibiliCookieUpsert,
  ClaudeCodeSettingsSaved,
  ClaudeKey,
  ClaudeKeyCreate,
  ClaudeKeyList,
  ClaudeKeyUpdate,
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


export const getListClaudeKeysUrl = () => {




  return `/api/v1/system/claude-keys`
}

/**
 * @summary 查询 Claude 网关密钥列表（安全视图，绝不含完整 Key）
 */
export const listClaudeKeys = async ( options?: RequestInit): Promise<ClaudeKeyList> => {

  return apiFetch<ClaudeKeyList>(getListClaudeKeysUrl(),
  {
    ...options,
    method: 'GET'


  }
);}


export const getCreateClaudeKeyUrl = () => {




  return `/api/v1/system/claude-keys`
}

/**
 * @summary 创建 Claude 网关密钥
 */
export const createClaudeKey = async (claudeKeyCreate: ClaudeKeyCreate, options?: RequestInit): Promise<ClaudeKey> => {

  return apiFetch<ClaudeKey>(getCreateClaudeKeyUrl(),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(claudeKeyCreate)
  }
);}


export const getUpdateClaudeKeyUrl = (keyId: string,) => {




  return `/api/v1/system/claude-keys/${keyId}`
}

/**
 * @summary 更新 Claude 网关密钥（省略 api_key 表示保留原 Key）
 */
export const updateClaudeKey = async (keyId: string,
    claudeKeyUpdate: ClaudeKeyUpdate, options?: RequestInit): Promise<ClaudeKey> => {

  return apiFetch<ClaudeKey>(getUpdateClaudeKeyUrl(keyId),
  {
    ...options,
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(claudeKeyUpdate)
  }
);}


export const getDeleteClaudeKeyUrl = (keyId: string,) => {




  return `/api/v1/system/claude-keys/${keyId}`
}

/**
 * @summary 删除 Claude 网关密钥
 */
export const deleteClaudeKey = async (keyId: string, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteClaudeKeyUrl(keyId),
  {
    ...options,
    method: 'DELETE'


  }
);}


export const getCreateClaudeKeyTestUrl = (keyId: string,) => {




  return `/api/v1/system/claude-keys/${keyId}/test`
}

/**
 * @summary 创建单个 Claude 网关密钥的连通性测试任务（Worker 后台执行）
 */
export const createClaudeKeyTest = async (keyId: string, options?: RequestInit): Promise<AsyncAcceptedResponse> => {

  return apiFetch<AsyncAcceptedResponse>(getCreateClaudeKeyTestUrl(keyId),
  {
    ...options,
    method: 'POST'


  }
);}


export const getSaveClaudeKeyToClaudeCodeSettingsUrl = (keyId: string,) => {




  return `/api/v1/system/claude-keys/${keyId}/claude-code-settings`
}

/**
 * @summary 将指定 Claude Key 保存到本机 Claude Code 设置
 */
export const saveClaudeKeyToClaudeCodeSettings = async (keyId: string, options?: RequestInit): Promise<ClaudeCodeSettingsSaved> => {

  return apiFetch<ClaudeCodeSettingsSaved>(getSaveClaudeKeyToClaudeCodeSettingsUrl(keyId),
  {
    ...options,
    method: 'POST'


  }
);}


export const getGetBilibiliCookieStatusUrl = () => {




  return `/api/v1/system/bilibili-cookie`
}

/**
 * @summary 查询 B 站 Cookie 状态（安全视图，绝不含完整 Cookie）
 */
export const getBilibiliCookieStatus = async ( options?: RequestInit): Promise<BilibiliCookieStatus> => {

  return apiFetch<BilibiliCookieStatus>(getGetBilibiliCookieStatusUrl(),
  {
    ...options,
    method: 'GET'


  }
);}


export const getSaveBilibiliCookieUrl = () => {




  return `/api/v1/system/bilibili-cookie`
}

/**
 * PUT 天然幂等（相同内容重复保存结果一致，写盘原子），不要求 Idempotency-Key。
 * @summary 保存 B 站 Cookie（Netscape 导出内容、Cookie 头串或 SESSDATA；仅保留 bilibili.com 条目）
 */
export const saveBilibiliCookie = async (bilibiliCookieUpsert: BilibiliCookieUpsert, options?: RequestInit): Promise<BilibiliCookieStatus> => {

  return apiFetch<BilibiliCookieStatus>(getSaveBilibiliCookieUrl(),
  {
    ...options,
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(bilibiliCookieUpsert)
  }
);}


export const getDeleteBilibiliCookieUrl = () => {




  return `/api/v1/system/bilibili-cookie`
}

/**
 * @summary 删除已保存的 B 站 Cookie
 */
export const deleteBilibiliCookie = async ( options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteBilibiliCookieUrl(),
  {
    ...options,
    method: 'DELETE'


  }
);}


export const getCreateBilibiliCookieTestUrl = () => {




  return `/api/v1/system/bilibili-cookie/tests`
}

/**
 * @summary 创建 B 站 Cookie 可用性检测任务（Worker 后台执行）
 */
export const createBilibiliCookieTest = async ( options?: RequestInit): Promise<AsyncAcceptedResponse> => {

  return apiFetch<AsyncAcceptedResponse>(getCreateBilibiliCookieTestUrl(),
  {
    ...options,
    method: 'POST'


  }
);}


