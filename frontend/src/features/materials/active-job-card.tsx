import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'

import { getJob } from '@/api/generated/endpoints/jobs/jobs'
import {
  invalidateDocumentsChanged,
  invalidateJobChanged,
  invalidateLessonChanged,
} from '@/api/invalidation'
import { presentJobError } from '@/api/problem-messages'
import { queryKeys } from '@/api/query-client'
import { ErrorState } from '@/components/ui/error-state'
import { SkeletonText } from '@/components/ui/skeleton'
import { isJobActive, JOB_KIND_LABEL, JOB_STATUS_META } from '@/lib/status'

/**
 * 课时进行中的生成任务卡（后台 Job 进度，与上传进度区分）：
 * 进度由全局 SSE 就地合并进任务缓存（不轮询）；任务进入终态时
 * 精确失效课时与任务缓存（SSE 终态对账之外的页面本地安全网，
 * 覆盖初始 REST 加载已完成任务等路径），让课时状态从
 * processing 流转为 ready/failed。
 */
export function ActiveJobCard({
  jobId,
  lessonId,
  courseId,
}: {
  jobId: string
  lessonId: string
  courseId: string
}) {
  const queryClient = useQueryClient()
  const settledRef = useRef<string | null>(null)

  const jobQuery = useQuery({
    queryKey: queryKeys.job(jobId),
    queryFn: () => getJob(jobId),
  })

  const status = jobQuery.data?.status
  useEffect(() => {
    if (status && !isJobActive(status) && settledRef.current !== status) {
      settledRef.current = status
      void invalidateLessonChanged(queryClient, lessonId, courseId)
      void invalidateJobChanged(queryClient, jobId)
      // 生成任务结束后文档可能已重写：阅读页收到精确失效后只提示、不打断阅读。
      void invalidateDocumentsChanged(queryClient, lessonId)
    }
  }, [status, queryClient, lessonId, courseId, jobId])

  if (jobQuery.isPending) {
    return (
      <div
        role="status"
        aria-busy="true"
        aria-label="任务状态加载中"
        className="rounded-md border border-border bg-surface p-4"
      >
        <SkeletonText lines={2} />
      </div>
    )
  }
  if (jobQuery.isError) {
    return (
      <ErrorState
        title="任务状态加载失败"
        error={jobQuery.error}
        onRetry={() => void jobQuery.refetch()}
      />
    )
  }

  const job = jobQuery.data
  const active = isJobActive(job.status)

  return (
    <section aria-label="后台生成任务" className="rounded-md border border-border bg-surface p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm font-medium text-text">
          {JOB_KIND_LABEL[job.kind]} · {JOB_STATUS_META[job.status].label}
        </p>
        <Link
          to="/jobs/$jobId"
          params={{ jobId: job.id }}
          className="rounded-sm text-sm text-accent hover:underline"
        >
          查看任务详情
        </Link>
      </div>
      {active ? (
        <>
          <div
            role="progressbar"
            aria-valuenow={job.progress}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label="生成任务进度"
            className="mt-2 h-1.5 w-full rounded-full bg-surface-sunken"
          >
            <div
              className="h-full rounded-full bg-info transition-[width] duration-180"
              style={{ width: `${job.progress}%` }}
            />
          </div>
          <p className="mt-1 text-sm text-text-muted">
            {job.stage} · {job.progress}% · 离开页面任务仍继续，进度也会显示在任务中心。
          </p>
        </>
      ) : (
        <p className="mt-1 text-sm text-text-muted">
          {job.status === 'succeeded'
            ? '生成完成，课时内容已就绪。'
            : job.error
              ? // code-first 呈现：不回显上游 detail（链接/Key/prompt）
                presentJobError(job.error.code, job.retryable).message
              : (job.message ?? '任务已结束。')}
        </p>
      )}
    </section>
  )
}
