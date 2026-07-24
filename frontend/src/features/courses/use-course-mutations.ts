import { useMutation, useQueryClient } from '@tanstack/react-query'

import { deleteCourse, updateCourse } from '@/api/generated/endpoints/courses/courses'
import type { CourseUpdate } from '@/api/generated/model'
import { invalidateCourseChanged, invalidateCourseRemoved } from '@/api/invalidation'

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
    onSuccess: () => invalidateCourseRemoved(queryClient, courseId),
  })
}
