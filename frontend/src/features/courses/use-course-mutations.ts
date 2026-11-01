import { useMutation, useQueryClient } from '@tanstack/react-query'

import {
  batchDeleteCourses,
  deleteCourse,
  updateCourse,
} from '@/api/generated/endpoints/courses/courses'
import type { CourseUpdate } from '@/api/generated/model'
import {
  invalidateCourseChanged,
  invalidateCourseRemoved,
  invalidateCoursesRemoved,
} from '@/api/invalidation'
import { removeRecentLearningEntries } from '@/features/learning/recent-learning'

/**
 * 课程更新/删除 Mutations。PATCH 携带 version 乐观锁，天然幂等，
 * 不需要 Idempotency-Key；version_conflict 由调用方按映射表呈现。
 */
export function useUpdateCourse(courseId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CourseUpdate) => updateCourse(courseId, input),
    onSuccess: () => invalidateCourseChanged(queryClient, courseId),
  })
}

export function useDeleteCourse(courseId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => deleteCourse(courseId),
    onSuccess: () => {
      removeRecentLearningEntries({ courseId })
      return invalidateCourseRemoved(queryClient, courseId)
    },
  })
}

/** 批量删除课程：原子接口（任一失败整体回滚）；成功后合并刷新共享查询。 */
export function useBatchDeleteCourses() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (ids: string[]) => batchDeleteCourses({ ids }),
    onSuccess: (_data, ids) => {
      ids.forEach((courseId) => removeRecentLearningEntries({ courseId }))
      return invalidateCoursesRemoved(queryClient, ids)
    },
  })
}
