/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  Progress
} from '../../model';

import { apiFetch } from '../../../http';

export const getGetCourseProgressUrl = (courseId: string,) => {




  return `/api/v1/courses/${courseId}/progress`
}

/**
 * @summary 查询课程进度聚合
 */
export const getCourseProgress = async (courseId: string, options?: RequestInit): Promise<Progress> => {

  return apiFetch<Progress>(getGetCourseProgressUrl(courseId),
  {
    ...options,
    method: 'GET'


  }
);}


export const getGetLessonProgressUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/progress`
}

/**
 * @summary 查询课时掌握度与趋势
 */
export const getLessonProgress = async (lessonId: string, options?: RequestInit): Promise<Progress> => {

  return apiFetch<Progress>(getGetLessonProgressUrl(lessonId),
  {
    ...options,
    method: 'GET'


  }
);}


