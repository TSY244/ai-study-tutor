/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  ListWeaknessesParams,
  ResourceVersion,
  Weakness,
  WeaknessCreate,
  WeaknessList,
  WeaknessUpdate
} from '../../model';

import { apiFetch } from '../../../http';

export const getListWeaknessesUrl = (params?: ListWeaknessesParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/weaknesses?${stringifiedParams}` : `/api/v1/weaknesses`
}

/**
 * @summary 查询薄弱点
 */
export const listWeaknesses = async (params?: ListWeaknessesParams, options?: RequestInit): Promise<WeaknessList> => {

  return apiFetch<WeaknessList>(getListWeaknessesUrl(params),
  {
    ...options,
    method: 'GET'


  }
);}


export const getGetWeaknessUrl = (weaknessId: string,) => {




  return `/api/v1/weaknesses/${weaknessId}`
}

/**
 * @summary 查询薄弱点及证据
 */
export const getWeakness = async (weaknessId: string, options?: RequestInit): Promise<Weakness> => {

  return apiFetch<Weakness>(getGetWeaknessUrl(weaknessId),
  {
    ...options,
    method: 'GET'


  }
);}


export const getUpdateWeaknessUrl = (weaknessId: string,) => {




  return `/api/v1/weaknesses/${weaknessId}`
}

/**
 * 携带当前 version，修改后重新计算课时/课程掌握度。
 * @summary 编辑薄弱点
 */
export const updateWeakness = async (weaknessId: string,
    weaknessUpdate: WeaknessUpdate, options?: RequestInit): Promise<Weakness> => {

  return apiFetch<Weakness>(getUpdateWeaknessUrl(weaknessId),
  {
    ...options,
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(weaknessUpdate)
  }
);}


export const getDeleteWeaknessUrl = (weaknessId: string,) => {




  return `/api/v1/weaknesses/${weaknessId}`
}

/**
 * @summary 删除薄弱点
 */
export const deleteWeakness = async (weaknessId: string, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteWeaknessUrl(weaknessId),
  {
    ...options,
    method: 'DELETE'


  }
);}


export const getResolveWeaknessUrl = (weaknessId: string,) => {




  return `/api/v1/weaknesses/${weaknessId}/resolve`
}

/**
 * @summary 手动标记薄弱点已掌握
 */
export const resolveWeakness = async (weaknessId: string,
    resourceVersion: ResourceVersion, options?: RequestInit): Promise<Weakness> => {

  return apiFetch<Weakness>(getResolveWeaknessUrl(weaknessId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(resourceVersion)
  }
);}


export const getReopenWeaknessUrl = (weaknessId: string,) => {




  return `/api/v1/weaknesses/${weaknessId}/reopen`
}

/**
 * @summary 重新激活薄弱点
 */
export const reopenWeakness = async (weaknessId: string,
    resourceVersion: ResourceVersion, options?: RequestInit): Promise<Weakness> => {

  return apiFetch<Weakness>(getReopenWeaknessUrl(weaknessId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(resourceVersion)
  }
);}


export const getCreateWeaknessUrl = (courseId: string,) => {




  return `/api/v1/courses/${courseId}/weaknesses`
}

/**
 * @summary 手工新增薄弱点
 */
export const createWeakness = async (courseId: string,
    weaknessCreate: WeaknessCreate, options?: RequestInit): Promise<Weakness> => {

  return apiFetch<Weakness>(getCreateWeaknessUrl(courseId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(weaknessCreate)
  }
);}


