import { useMutation, useQueryClient } from '@tanstack/react-query'

import { submitAttempt } from '@/api/generated/endpoints/attempts/attempts'
import type { AttemptSubmitAccepted, AttemptSubmitCompleted } from '@/api/generated/model'
import { invalidateJobChanged } from '@/api/invalidation'
import { isApiProblemError, isProblemCode } from '@/api/problem'
import { queryKeys } from '@/api/query-client'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

/** 提交意图 scope（每个 Attempt 一个）：连点/网络重试复用同一 Key。 */
export function submitAttemptIntentScope(attemptId: string): string {
  return `attempt-submit:${attemptId}`
}

/**
 * 提交作答（契约 §10.4）：
 * - 携带当前 attempt_version；version_conflict 时用服务端 current_version
 *   轮换幂等键后自动重试一次（载荷变化必须换 Key，否则 idempotency_conflict）。
 * - 200 AttemptSubmitCompleted（全客观题，无 job）与 202 AttemptSubmitAccepted
 *   （必有 attempt_grade job）按生成类型判别：'job' in result——契约保证
 *   202 的 job 非空、200 的 schema 不允许 job 字段，不靠可空字段猜测。
 * - 成功：结果写回 attempt 缓存（路由随即切到结果视图，作答锁定）；
 *   202 同时把 job 写入缓存并失效任务中心。
 */
export function useSubmitAttempt(attemptId: string) {
  const queryClient = useQueryClient()
  const scope = submitAttemptIntentScope(attemptId)

  return useMutation({
    mutationFn: async ({ version }: { version: number }) => {
      try {
        return await submitAttempt(
          attemptId,
          { version },
          { headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) } },
        )
      } catch (error) {
        if (isApiProblemError(error) && error.code === 'version_conflict') {
          const currentVersion = error.getExtension<number>('current_version')
          if (typeof currentVersion === 'number') {
            rotateIdempotencyIntent(scope)
            return await submitAttempt(
              attemptId,
              { version: currentVersion },
              { headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) } },
            )
          }
        }
        throw error
      }
    },
    onSuccess: async (result: AttemptSubmitCompleted | AttemptSubmitAccepted) => {
      resolveIdempotencyIntent(scope)
      queryClient.setQueryData(queryKeys.attempt(attemptId), result.attempt)
      if ('job' in result) {
        queryClient.setQueryData(queryKeys.job(result.job.id), result.job)
        await invalidateJobChanged(queryClient, result.job.id)
      }
    },
    onError: (error) => {
      if (isProblemCode(error, 'idempotency_conflict')) rotateIdempotencyIntent(scope)
      // 已提交：拉取最新状态，路由会切到结果视图
      if (isProblemCode(error, 'attempt_already_submitted')) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.attempt(attemptId) })
      }
    },
  })
}
