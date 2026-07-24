/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  Course,
  CourseCreate,
  CourseList,
  CourseUpdate,
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


