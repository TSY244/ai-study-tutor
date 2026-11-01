import { useMutation, useQueryClient } from '@tanstack/react-query'

import {
  batchDeleteLessons,
  createLesson,
  deleteLesson,
  updateLesson,
} from '@/api/generated/endpoints/lessons/lessons'
import type { LessonCreate, LessonUpdate } from '@/api/generated/model'
import {
  invalidateLessonChanged,
  invalidateLessonCreated,
  invalidateLessonRemoved,
  invalidateLessonsRemoved,
} from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import { removeRecentLearningEntries } from '@/features/learning/recent-learning'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

/** 创建课时意图 scope（每门课程一个）：重试/双击/刷新恢复复用同一 Key。 */
export function createLessonIntentScope(courseId: string): string {
  return `create-lesson:${courseId}`
}

/**
 * 创建课时 Mutation：Idempotency-Key 生命周期与创建课程一致——
 * 成功结束意图；validation_error/idempotency_conflict（载荷必变）轮换 Key；
 * 网络类与可重试错误保留同一 Key。成功后精确失效课时/课程/进度。
 */
export function useCreateLesson(courseId: string) {
  const queryClient = useQueryClient()
  const scope = createLessonIntentScope(courseId)
  return useMutation({
    mutationFn: (input: LessonCreate) =>
      createLesson(courseId, input, {
        headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) },
      }),
    onSuccess: async () => {
      resolveIdempotencyIntent(scope)
      await invalidateLessonCreated(queryClient, courseId)
    },
    onError: (error) => {
      if (
        isApiProblemError(error) &&
        (error.code === 'validation_error' || error.code === 'idempotency_conflict')
      ) {
        rotateIdempotencyIntent(scope)
      }
    },
  })
}

/** 更新课时：PATCH 携带 version 乐观锁，天然幂等；冲突按映射表呈现。 */
export function useUpdateLesson(lessonId: string, courseId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: LessonUpdate) => updateLesson(lessonId, input),
    onSuccess: () => invalidateLessonChanged(queryClient, lessonId, courseId),
  })
}

export function useDeleteLesson(lessonId: string, courseId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => deleteLesson(lessonId),
    onSuccess: () => {
      removeRecentLearningEntries({ lessonId })
      return invalidateLessonRemoved(queryClient, lessonId, courseId)
    },
  })
}

/** 批量删除课时：原子接口（任一失败整体回滚）；成功后合并刷新共享查询。 */
export function useBatchDeleteLessons(courseId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (ids: string[]) => batchDeleteLessons({ ids }),
    onSuccess: (_data, ids) => {
      ids.forEach((lessonId) => removeRecentLearningEntries({ lessonId }))
      return invalidateLessonsRemoved(queryClient, ids, courseId)
    },
  })
}
