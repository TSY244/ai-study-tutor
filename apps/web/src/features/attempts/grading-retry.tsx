import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'

import { createAttemptGradingJob } from '@/api/generated/endpoints/attempts/attempts'
import { getJob } from '@/api/generated/endpoints/jobs/jobs'
import type { Attempt, Job } from '@/api/generated/model'
import { invalidateJobChanged } from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import { getResourceBusyInfo, presentProblem } from '@/api/problem-messages'
import { queryKeys } from '@/api/query-client'
import { Button } from '@/components/ui/button'
import { FormAlert } from '@/components/ui/form-alert'
import { useRetryJob } from '@/features/jobs/use-job-actions'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

/**
 * grading_failed 的恢复入口（handoff §6.5 / 契约 §10.5）：
 * - 有失败 Job 且 retryable → 走 Job Retry（新 Job retry_of_job_id 指回原 Job）。
 * - 无 Job / 旧数据 / 不可重试 → POST /attempts/{id}/grading-jobs。
 * - 两者复用同一次用户意图的幂等键；resource_busy（已有活动批阅任务）
 *   时给出该任务链接，不换 Key 重建。
 */
export function GradingRetry({ attempt }: { attempt: Attempt }) {
  const gradingJobQuery = useQuery({
    queryKey: queryKeys.job(attempt.grading_job_id ?? ''),
    queryFn: () => getJob(attempt.grading_job_id as string),
    enabled: Boolean(attempt.grading_job_id),
  })
  const failedJob =
    gradingJobQuery.data && gradingJobQuery.data.status === 'failed'
      ? gradingJobQuery.data
      : undefined

  return (
    <section aria-label="重新批阅" className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        {failedJob?.retryable ? (
          <RetryFailedJobButton job={failedJob} attemptId={attempt.id} />
        ) : (
          <CreateGradingJobButton attemptId={attempt.id} />
        )}
        {attempt.grading_job_id ? (
          <Link
            to="/jobs/$jobId"
            params={{ jobId: attempt.grading_job_id }}
            className="rounded-sm text-sm text-accent hover:underline"
          >
            查看批阅任务
          </Link>
        ) : null}
      </div>
      {failedJob && !failedJob.retryable ? (
        <p className="text-sm text-text-muted">
          原批阅任务不可重试{failedJob.error?.detail ? `：${failedJob.error.detail}` : '。'}
          请使用「重新批阅」创建新的批阅任务。
        </p>
      ) : null}
    </section>
  )
}

/** 分支一：重试失败的批阅 Job（复用 F3 的 useRetryJob 与幂等意图）。 */
function RetryFailedJobButton({ job, attemptId }: { job: Job; attemptId: string }) {
  const queryClient = useQueryClient()
  const retryJob = useRetryJob(job)
  const [error, setError] = useState<unknown>(null)

  return (
    <>
      <Button
        variant="primary"
        size="sm"
        disabled={retryJob.isPending}
        onClick={() => {
          setError(null)
          retryJob.mutate(undefined, {
            onSuccess: async () => {
              await queryClient.invalidateQueries({ queryKey: queryKeys.attempt(attemptId) })
            },
            onError: setError,
          })
        }}
      >
        {retryJob.isPending ? '重试中…' : '重试批阅任务'}
      </Button>
      {error ? <RetryError error={error} /> : null}
    </>
  )
}

/** 分支二：Attempt 级重新批阅（无 Job / 旧数据 / 不可重试时使用）。 */
function CreateGradingJobButton({ attemptId }: { attemptId: string }) {
  const queryClient = useQueryClient()
  const [error, setError] = useState<unknown>(null)
  const scope = `grading-retry:${attemptId}`

  const createGradingJob = useMutation({
    mutationFn: () =>
      createAttemptGradingJob(attemptId, {
        headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) },
      }),
    onSuccess: async (accepted) => {
      resolveIdempotencyIntent(scope)
      setError(null)
      queryClient.setQueryData(queryKeys.job(accepted.job.id), accepted.job)
      await invalidateJobChanged(queryClient, accepted.job.id)
      await queryClient.invalidateQueries({ queryKey: queryKeys.attempt(attemptId) })
    },
    onError: (mutationError) => {
      if (isApiProblemError(mutationError) && mutationError.code === 'idempotency_conflict') {
        rotateIdempotencyIntent(scope)
      }
      setError(mutationError)
    },
  })

  return (
    <>
      <Button
        variant="primary"
        size="sm"
        disabled={createGradingJob.isPending}
        onClick={() => {
          setError(null)
          createGradingJob.mutate()
        }}
      >
        {createGradingJob.isPending ? '创建中…' : '重新批阅'}
      </Button>
      {error ? <RetryError error={error} /> : null}
    </>
  )
}

function RetryError({ error }: { error: unknown }) {
  const busyInfo = getResourceBusyInfo(error)
  if (busyInfo?.activeJobId) {
    return (
      <p role="alert" className="text-sm text-text-muted">
        已有进行中的批阅任务，
        <Link
          to="/jobs/$jobId"
          params={{ jobId: busyInfo.activeJobId }}
          className="rounded-sm text-accent hover:underline"
        >
          查看该任务
        </Link>
        。
      </p>
    )
  }
  return (
    <FormAlert
      title={presentProblem(error).title}
      description={presentProblem(error).message}
      code={isApiProblemError(error) ? error.code : undefined}
      requestId={isApiProblemError(error) ? error.requestId : undefined}
    />
  )
}
