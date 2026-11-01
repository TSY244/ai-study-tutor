import { Link } from '@tanstack/react-router'

import { Badge } from '@/components/ui/badge'
import { ErrorState } from '@/components/ui/error-state'
import { SkeletonText } from '@/components/ui/skeleton'
import type { useActiveResourceJobs } from '@/features/jobs/use-active-resource-jobs'
import { JOB_KIND_LABEL, JOB_STATUS_META } from '@/lib/status'

export function ResourceJobs({
  query,
  title,
}: {
  query: ReturnType<typeof useActiveResourceJobs>
  title: string
}) {
  if (query.isLoading) {
    return (
      <div role="status" aria-label="进行中的任务加载中" aria-busy="true">
        <SkeletonText lines={2} />
      </div>
    )
  }
  if (query.isError) {
    return (
      <ErrorState
        title="进行中的任务加载失败"
        error={query.error}
        onRetry={() => void query.refetch()}
      />
    )
  }
  if (query.jobs.length === 0) return null

  return (
    <section aria-label={title} className="space-y-3">
      <h2 className="text-sm font-medium text-text">{title}</h2>
      <ul className="space-y-3">
        {query.jobs.map((job) => (
          <li key={job.id} className="rounded-md border border-info/30 bg-info-bg p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="flex items-center gap-2 text-sm font-medium text-text">
                {JOB_KIND_LABEL[job.kind]}
                <Badge variant={JOB_STATUS_META[job.status].variant}>
                  {JOB_STATUS_META[job.status].label}
                </Badge>
              </p>
              <Link
                to="/jobs/$jobId"
                params={{ jobId: job.id }}
                className="rounded-sm text-sm text-accent hover:underline"
              >
                查看任务详情
              </Link>
            </div>
            <div
              role="progressbar"
              aria-label={`${JOB_KIND_LABEL[job.kind]}进度`}
              aria-valuenow={job.progress}
              aria-valuemin={0}
              aria-valuemax={100}
              className="mt-3 h-1.5 rounded-full bg-surface-sunken"
            >
              <div
                className="h-full rounded-full bg-info transition-[width] duration-180"
                style={{ width: `${job.progress}%` }}
              />
            </div>
            <p className="mt-2 text-sm text-text-muted">
              {job.stage} · {job.progress}%
            </p>
            {job.message ? (
              <p role="status" className="mt-1 text-sm text-text-muted">
                {job.message}
              </p>
            ) : null}
            <p className="mt-1 text-xs text-text-subtle">
              离开页面任务仍会继续，完成后这里会自动更新。
            </p>
          </li>
        ))}
      </ul>
    </section>
  )
}
