/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  Lesson,
  LessonCreate,
  LessonList,
  LessonUpdate,
  ListLessonsParams
} from '../../model';

import { apiFetch } from '../../../http';

export const getListLessonsUrl = (courseId: string,
    params?: ListLessonsParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/courses/${courseId}/lessons?${stringifiedParams}` : `/api/v1/courses/${courseId}/lessons`
}

/**
 * @summary 查询课程下的课时
 */
export const listLessons = async (courseId: string,
    params?: ListLessonsParams, options?: RequestInit): Promise<LessonList> => {

  return apiFetch<LessonList>(getListLessonsUrl(courseId,params),
  {
    ...options,
    method: 'GET'


  }
);}


export const getCreateLessonUrl = (courseId: string,) => {




  return `/api/v1/courses/${courseId}/lessons`
}

/**
 * @summary 创建空课时
 */
export const createLesson = async (courseId: string,
    lessonCreate: LessonCreate, options?: RequestInit): Promise<Lesson> => {

  return apiFetch<Lesson>(getCreateLessonUrl(courseId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(lessonCreate)
  }
);}


export const getGetLessonUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}`
}

/**
 * @summary 查询课时详情
 */
export const getLesson = async (lessonId: string, options?: RequestInit): Promise<Lesson> => {

  return apiFetch<Lesson>(getGetLessonUrl(lessonId),
  {
    ...options,
    method: 'GET'


  }
);}


export const getUpdateLessonUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}`
}

/**
 * @summary 更新课时
 */
export const updateLesson = async (lessonId: string,
    lessonUpdate: LessonUpdate, options?: RequestInit): Promise<Lesson> => {

  return apiFetch<Lesson>(getUpdateLessonUrl(lessonId),
  {
    ...options,
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(lessonUpdate)
  }
);}


export const getDeleteLessonUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}`
}

/**
 * @summary 删除课时
 */
export const deleteLesson = async (lessonId: string, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteLessonUrl(lessonId),
  {
    ...options,
    method: 'DELETE'


  }
);}


