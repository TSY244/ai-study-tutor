import { useMutation, useQueryClient } from '@tanstack/react-query'

import { createCourse } from '@/api/generated/endpoints/courses/courses'
import type { CourseCreate } from '@/api/generated/model'
import { invalidateCoursesList } from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

/** 创建课程意图 scope：同一意图的重试/双击/刷新恢复复用同一 Idempotency-Key。 */
export const CREATE_COURSE_INTENT_SCOPE = 'create-course'

/**
 * 创建课程 Mutation（契约 04-api-contract.md §2.3 的 Key 生命周期）：
 * - 提交时 begin：复用 sessionStorage 中该意图的 Key；
 * - 成功：resolve 结束意图（下一门课程是新意图），并精确失效课程列表；
 * - validation_error / idempotency_conflict：载荷必须变化 → rotate 换新 Key，
 *   避免同一 Key 携带不同内容被服务端判为冲突；
 * - network_error / 5xx / idempotency_in_progress：保留同一 Key 供重试复用。
 */
export function useCreateCourse() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CourseCreate) =>
      createCourse(input, {
        headers: { 'Idempotency-Key': beginIdempotencyIntent(CREATE_COURSE_INTENT_SCOPE) },
      }),
    onSuccess: async () => {
      resolveIdempotencyIntent(CREATE_COURSE_INTENT_SCOPE)
      await invalidateCoursesList(queryClient)
    },
    onError: (error) => {
      if (
        isApiProblemError(error) &&
        (error.code === 'validation_error' || error.code === 'idempotency_conflict')
      ) {
        rotateIdempotencyIntent(CREATE_COURSE_INTENT_SCOPE)
      }
    },
  })
}
