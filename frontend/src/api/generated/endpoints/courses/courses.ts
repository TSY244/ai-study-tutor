/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  BatchDelete,
  Course,
  CourseCreate,
  CourseList,
  CourseUpdate,
  KnowledgePoint,
  KnowledgePointCollection,
  KnowledgePointCreate,
  KnowledgePointUpdate,
  ListCourseKnowledgePointsParams,
  ListCoursesParams
} from '../../model';

import { apiFetch } from '../../../http';

export const getListCoursesUrl = (params?: ListCoursesParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/courses?${stringifiedParams}` : `/api/v1/courses`
}

/**
 * @summary 查询课程列表
 */
export const listCourses = async (params?: ListCoursesParams, options?: RequestInit): Promise<CourseList> => {

  return apiFetch<CourseList>(getListCoursesUrl(params),
  {
    ...options,
    method: 'GET'


  }
);}


export const getCreateCourseUrl = () => {




  return `/api/v1/courses`
}

/**
 * @summary 创建课程
 */
export const createCourse = async (courseCreate: CourseCreate, options?: RequestInit): Promise<Course> => {

  return apiFetch<Course>(getCreateCourseUrl(),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(courseCreate)
  }
);}


export const getBatchDeleteCoursesUrl = () => {




  return `/api/v1/courses:batch-delete`
}

/**
 * @summary 批量删除课程（原子：任一课程不存在或有进行中任务则整体回滚）
 */
export const batchDeleteCourses = async (batchDelete: BatchDelete, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getBatchDeleteCoursesUrl(),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(batchDelete)
  }
);}


export const getGetCourseUrl = (courseId: string,) => {




  return `/api/v1/courses/${courseId}`
}

/**
 * @summary 查询课程详情
 */
export const getCourse = async (courseId: string, options?: RequestInit): Promise<Course> => {

  return apiFetch<Course>(getGetCourseUrl(courseId),
  {
    ...options,
    method: 'GET'


  }
);}


export const getUpdateCourseUrl = (courseId: string,) => {




  return `/api/v1/courses/${courseId}`
}

/**
 * @summary 更新课程
 */
export const updateCourse = async (courseId: string,
    courseUpdate: CourseUpdate, options?: RequestInit): Promise<Course> => {

  return apiFetch<Course>(getUpdateCourseUrl(courseId),
  {
    ...options,
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(courseUpdate)
  }
);}


export const getDeleteCourseUrl = (courseId: string,) => {




  return `/api/v1/courses/${courseId}`
}

/**
 * @summary 删除课程
 */
export const deleteCourse = async (courseId: string, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteCourseUrl(courseId),
  {
    ...options,
    method: 'DELETE'


  }
);}


export const getListCourseKnowledgePointsUrl = (courseId: string,
    params?: ListCourseKnowledgePointsParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/courses/${courseId}/knowledge-points?${stringifiedParams}` : `/api/v1/courses/${courseId}/knowledge-points`
}

/**
 * @summary 查看课程知识点及原文依据
 */
export const listCourseKnowledgePoints = async (courseId: string,
    params?: ListCourseKnowledgePointsParams, options?: RequestInit): Promise<KnowledgePointCollection> => {

  return apiFetch<KnowledgePointCollection>(getListCourseKnowledgePointsUrl(courseId,params),
  {
    ...options,
    method: 'GET'


  }
);}


export const getCreateKnowledgePointUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/knowledge-points`
}

/**
 * @summary 新增课时知识点
 */
export const createKnowledgePoint = async (lessonId: string,
    knowledgePointCreate: KnowledgePointCreate, options?: RequestInit): Promise<KnowledgePoint> => {

  return apiFetch<KnowledgePoint>(getCreateKnowledgePointUrl(lessonId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(knowledgePointCreate)
  }
);}


export const getGetKnowledgePointUrl = (knowledgePointId: string,) => {




  return `/api/v1/knowledge-points/${knowledgePointId}`
}

/**
 * @summary 查询单个知识点
 */
export const getKnowledgePoint = async (knowledgePointId: string, options?: RequestInit): Promise<KnowledgePoint> => {

  return apiFetch<KnowledgePoint>(getGetKnowledgePointUrl(knowledgePointId),
  {
    ...options,
    method: 'GET'


  }
);}


export const getUpdateKnowledgePointUrl = (knowledgePointId: string,) => {




  return `/api/v1/knowledge-points/${knowledgePointId}`
}

/**
 * @summary 修改知识点
 */
export const updateKnowledgePoint = async (knowledgePointId: string,
    knowledgePointUpdate: KnowledgePointUpdate, options?: RequestInit): Promise<KnowledgePoint> => {

  return apiFetch<KnowledgePoint>(getUpdateKnowledgePointUrl(knowledgePointId),
  {
    ...options,
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(knowledgePointUpdate)
  }
);}


export const getDeleteKnowledgePointUrl = (knowledgePointId: string,) => {




  return `/api/v1/knowledge-points/${knowledgePointId}`
}

/**
 * 历史试卷中的知识点快照保留。
 * @summary 删除知识点
 */
export const deleteKnowledgePoint = async (knowledgePointId: string, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteKnowledgePointUrl(knowledgePointId),
  {
    ...options,
    method: 'DELETE'


  }
);}


