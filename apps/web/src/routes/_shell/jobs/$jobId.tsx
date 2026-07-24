import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { CircleAlert } from 'lucide-react'

import { getJob } from '@/api/generated/endpoints/jobs/jobs'
import { getLesson } from '@/api/generated/endpoints/lessons/lessons'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { ErrorState } from '@/components/ui/error-state'
import { PageHeader } from '@/components/ui/page-header'
import { PathLink } from '@/components/ui/path-link'
import { Skeleton, SkeletonText } from '@/components/ui/skeleton'
import { JobActions } from '@/features/jobs/job-actions'
import { JobEventsSection } from '@/features/jobs/job-events-section'
import { formatDateTime } from '@/lib/format'
import { isJobActive, JOB_KIND_LABEL, JOB_STATUS_META } from '@/lib/status'

/**
 * 任务详情：状态/阶段/进度/错误/关联对象 + 取消/重试操作 + 事件时间线。
 * 进行中任务由全局 SSE 就地合并更新（不轮询），断线时由 SSE 客户端
 * REST 对账兜底；时间线在新事件到达时精准失效。
 */
export const Route = createFileRoute('/_shell/jobs/$jobId')({
  staticData: { crumb: '任务详情' },
  component: JobDetailPage,
})

function JobDetailPage() {
  const { jobId } = Route.useParams()
  const jobQuery = useQuery({
    queryKey: queryKeys.job(jobId),
    queryFn: () => getJob(jobId),
  })

  if (jobQuery.isPending) {
    return (
      <div role="status" aria-busy="true" aria-label="任务加载中" className="space-y-5">
        <Skeleton className="h-6 w-48" />
        <SkeletonText lines={4} />
      </div>
    )
  }

  if (jobQuery.isError) {
    return (
      <ErrorState
        title="任务加载失败"
        error={jobQuery.error}
        onRetry={() => void jobQuery.refetch()}
      />
    )
  }

  const job = jobQuery.data
  const statusMeta = JOB_STATUS_META[job.status]
  const active = isJobActive(job.status)

  return (
    <div className="max-w-2xl space-y-5">
      <PageHeader
        title={JOB_KIND_LABEL[job.kind]}
        actions={<Badge variant={statusMeta.variant}>{statusMeta.label}</Badge>}
      />

      {active || job.status === 'succeeded' ? (
        <section aria-label="任务进度">
          <div
            role="progressbar"
            aria-valuenow={job.progress}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label={`${JOB_KIND_LABEL[job.kind]}进度`}
            className="h-2 w-full rounded-full bg-surface-sunken"
          >
            <div
              className={`h-full rounded-full transition-[width] duration-180 ${job.status === 'succeeded' ? 'bg-ok' : 'bg-info'}`}
              style={{ width: `${job.progress}%` }}
            />
          </div>
          <p className="mt-1 text-sm text-text-muted">
            {job.stage} · {job.progress}%
          </p>
        </section>
      ) : null}

      {job.error ? (
        <div role="alert" className="rounded-md border border-border-strong bg-danger-bg p-4">
          <p className="flex items-center gap-2 font-medium text-danger">
            <CircleAlert aria-hidden="true" className="h-4 w-4 shrink-0" />
            任务失败
          </p>
          <p className="mt-1 text-sm text-text">{job.error.detail}</p>
          <p className="mt-2 text-sm text-text-subtle">
            错误码 <span className="font-mono">{job.error.code}</span> ·{' '}
            {job.retryable ? '可重试' : '不可重试'}
            {job.attempt_count > 1 ? ` · 第 ${job.attempt_count}/${job.max_attempts} 次尝试` : ''}
          </p>
        </div>
      ) : null}

      <JobActions job={job} />

      <JobEventsSection jobId={job.id} />

      <dl className="divide-y divide-border rounded-md border border-border bg-surface text-sm">
        <DetailRow label="任务 ID">
          <span className="font-mono break-all">{job.id}</span>
        </DetailRow>
        <DetailRow label="当前阶段">{job.stage}</DetailRow>
        {job.message ? <DetailRow label="消息">{job.message}</DetailRow> : null}
        <DetailRow label="创建于">{formatDateTime(job.created_at)}</DetailRow>
        <DetailRow label="更新于">{formatDateTime(job.updated_at)}</DetailRow>
        {job.completed_at ? (
          <DetailRow label="完成于">{formatDateTime(job.completed_at)}</DetailRow>
        ) : null}
        {job.retry_of_job_id ? (
          <DetailRow label="重试自">
            <Link
              to="/jobs/$jobId"
              params={{ jobId: job.retry_of_job_id }}
              className="rounded-sm font-mono break-all text-accent hover:underline"
            >
              {job.retry_of_job_id}
            </Link>
          </DetailRow>
        ) : null}
        {job.aggregate_type === 'lesson' && job.aggregate_id ? (
          <DetailRow label="关联课时">
            <LessonLink lessonId={job.aggregate_id} />
          </DetailRow>
        ) : null}
      </dl>
    </div>
  )
}

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 px-4 py-2.5">
      <dt className="shrink-0 text-text-muted">{label}</dt>
      <dd className="min-w-0 text-right text-text">{children}</dd>
    </div>
  )
}

/** aggregate_id -> 课时链接（课时标题就绪后替换占位文本）。 */
function LessonLink({ lessonId }: { lessonId: string }) {
  const lessonQuery = useQuery({
    queryKey: queryKeys.lesson(lessonId),
    queryFn: () => getLesson(lessonId),
  })

  if (lessonQuery.isSuccess) {
    const lesson = lessonQuery.data
    return (
      <PathLink
        to={`/courses/${lesson.course_id}/lessons/${lesson.id}/notes`}
        className="rounded-sm text-accent hover:underline"
      >
        {lesson.title}
      </PathLink>
    )
  }
  return <span className="font-mono break-all">{lessonId}</span>
}
