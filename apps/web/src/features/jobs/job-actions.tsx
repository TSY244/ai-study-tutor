import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import type { Job } from '@/api/generated/model'
import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { useCancelJob, useRetryJob } from '@/features/jobs/use-job-actions'
import { isJobActive } from '@/lib/status'

/**
 * 任务操作（02 §6.2 状态矩阵）：
 * - 进行中（且未请求取消）：可取消（确认框）；
 * - 失败且 retryable：可重试（复用同一意图的幂等键，202 后跳转到新任务）；
 * - 失败不可重试：禁用重试并展示原因；409 job_not_retryable 同样落到这里。
 */
export function JobActions({ job }: { job: Job }) {
  const navigate = useNavigate()
  const [cancelOpen, setCancelOpen] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const cancelJob = useCancelJob(job)
  const retryJob = useRetryJob(job)

  const cancellable = isJobActive(job.status) && !job.cancel_requested
  const retryable = job.status === 'failed' && job.retryable
  const notRetryableReason =
    job.status === 'failed' && !job.retryable
      ? (job.error?.detail ?? '该任务的失败类型不支持重试。')
      : null

  const confirmCancel = async () => {
    setActionError(null)
    try {
      await cancelJob.mutateAsync()
      setCancelOpen(false)
    } catch (error) {
      const presentation = presentProblem(error)
      setActionError(`${presentation.title}：${presentation.message}`)
    }
  }

  const runRetry = async () => {
    setActionError(null)
    try {
      const accepted = await retryJob.mutateAsync()
      await navigate({ to: '/jobs/$jobId', params: { jobId: accepted.job.id } })
    } catch (error) {
      // job_not_retryable 等：留在详情页展示映射原因，状态不会因重试改变
      const presentation = presentProblem(error)
      setActionError(`${presentation.title}：${presentation.message}`)
    }
  }

  if (!cancellable && job.status !== 'failed') return null

  return (
    <section aria-label="任务操作" className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        {cancellable ? (
          <Button variant="secondary" size="sm" onClick={() => setCancelOpen(true)}>
            取消任务
          </Button>
        ) : null}
        {retryable ? (
          <Button size="sm" disabled={retryJob.isPending} onClick={() => void runRetry()}>
            {retryJob.isPending ? '重试中…' : '重试任务'}
          </Button>
        ) : null}
        {notRetryableReason ? (
          <p className="text-sm text-text-muted">
            <span className="font-medium text-text">不可重试</span>（{notRetryableReason}）
          </p>
        ) : null}
      </div>
      {actionError ? (
        <p role="alert" className="rounded-md bg-danger-bg px-3 py-2 text-sm text-danger">
          {actionError}
        </p>
      ) : null}

      <ConfirmDialog
        open={cancelOpen}
        onOpenChange={setCancelOpen}
        title="取消任务"
        description="取消后任务停止执行；已产生的中间数据保留。此操作不能撤销。"
        confirmLabel="取消任务"
        danger
        busy={cancelJob.isPending}
        errorMessage={actionError ?? undefined}
        onConfirm={() => void confirmCancel()}
      />
    </section>
  )
}
