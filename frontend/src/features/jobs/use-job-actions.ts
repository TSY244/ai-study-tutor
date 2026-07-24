import { useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query'

import { cancelJob, retryJob } from '@/api/generated/endpoints/jobs/jobs'
import type { Job } from '@/api/generated/model'
import { invalidateJobChanged } from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'
import { queryKeys } from '@/api/query-client'

/** 重试任务意图 scope（每个失败任务一个）：同一重试意图的连点/刷新恢复复用同一 Key。 */
export function retryJobIntentScope(jobId: string): string {
  return `retry-job:${jobId}`
}

/** 取消任务意图 scope：网络重试/重复确认复用同一 Key。 */
export function cancelJobIntentScope(jobId: string): string {
  return `cancel-job:${jobId}`
}

/** 任务关联的课时缓存也要刷新（取消/重试会改变课时状态）。 */
async function invalidateJobAndAggregate(client: QueryClient, job: Job) {
  await invalidateJobChanged(client, job.id)
  if (job.aggregate_type === 'lesson' && job.aggregate_id) {
    await Promise.all([
      client.invalidateQueries({ queryKey: queryKeys.lesson(job.aggregate_id) }),
      client.invalidateQueries({ queryKey: ['lessons'] }),
    ])
  }
}

/** 取消任务：进行中才可取消；POST 通过意图键与服务端状态机共同去重。 */
export function useCancelJob(job: Job) {
  const queryClient = useQueryClient()
  const scope = cancelJobIntentScope(job.id)
  return useMutation({
    mutationFn: () =>
      cancelJob(job.id, { headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) } }),
    onSuccess: async () => {
      resolveIdempotencyIntent(scope)
      await invalidateJobAndAggregate(queryClient, job)
    },
    onError: (error) => {
      if (isApiProblemError(error) && error.code === 'idempotency_conflict') {
        rotateIdempotencyIntent(scope)
      }
    },
  })
}

/**
 * 重试失败任务（02 §6.5：复用同一次用户意图的幂等键）：
 * 202 返回新任务（retry_of_job_id 指回原任务）；job_not_retryable 时
 * 保留 Key 且由 UI 禁用重试——该状态不会因重试改变。
 */
export function useRetryJob(job: Job) {
  const queryClient = useQueryClient()
  const scope = retryJobIntentScope(job.id)
  return useMutation({
    mutationFn: () =>
      retryJob(job.id, { headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) } }),
    onSuccess: async (accepted) => {
      resolveIdempotencyIntent(scope)
      await invalidateJobChanged(queryClient, job.id)
      await invalidateJobChanged(queryClient, accepted.job.id)
      return accepted
    },
    onError: (error) => {
      if (isApiProblemError(error) && error.code === 'idempotency_conflict') {
        rotateIdempotencyIntent(scope)
      }
    },
  })
}
