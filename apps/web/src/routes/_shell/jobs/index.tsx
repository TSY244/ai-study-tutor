import { useInfiniteQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { ChevronRight, ListTodo } from 'lucide-react'

import { listJobs } from '@/api/generated/endpoints/jobs/jobs'
import type { Job } from '@/api/generated/model'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { PageHeader } from '@/components/ui/page-header'
import { SkeletonRows } from '@/components/ui/skeleton'
import { formatDateTime } from '@/lib/format'
import { isJobActive, JOB_KIND_LABEL, JOB_STATUS_META } from '@/lib/status'

const PAGE_SIZE = 20

/**
 * 任务中心列表：游标分页（page_info.next_cursor），“加载更多”追加。
 * 取消/重试操作在 F3 接入；实时刷新由全局任务事件流（F6 SSE）驱动。
 */
export const Route = createFileRoute('/_shell/jobs/')({
  component: JobsPage,
})

function JobsPage() {
  const jobsQuery = useInfiniteQuery({
    queryKey: queryKeys.jobs({ view: 'paged' }),
    queryFn: ({ pageParam }) => listJobs({ cursor: pageParam || undefined, limit: PAGE_SIZE }),
    initialPageParam: '',
    getNextPageParam: (lastPage) =>
      lastPage.page_info.has_more ? (lastPage.page_info.next_cursor ?? undefined) : undefined,
  })

  const jobs = jobsQuery.data?.pages.flatMap((page) => page.items) ?? []

  return (
    <div className="space-y-5">
      <PageHeader
        title="任务中心"
        description="所有后台任务（导入、生成、批阅等）的进度与结果。离开页面任务仍继续运行。"
      />

      {jobsQuery.isPending ? (
        <div role="status" aria-busy="true" aria-label="任务列表加载中">
          <SkeletonRows rows={4} />
        </div>
      ) : null}

      {jobsQuery.isError ? (
        <ErrorState
          title="任务列表加载失败"
          error={jobsQuery.error}
          onRetry={() => void jobsQuery.refetch()}
        />
      ) : null}

      {jobsQuery.isSuccess ? (
        jobs.length === 0 ? (
          <EmptyState
            icon={ListTodo}
            title="暂无任务"
            description="导入课时、生成测验或提交批阅后，任务会出现在这里。"
            action={
              <Link
                to="/courses"
                className="rounded-sm text-sm font-medium text-accent hover:underline"
              >
                去课程看看
              </Link>
            }
          />
        ) : (
          <>
            <ul className="divide-y divide-border rounded-md border border-border bg-surface">
              {jobs.map((job) => (
                <JobRow key={job.id} job={job} />
              ))}
            </ul>
            {jobsQuery.hasNextPage ? (
              <div className="flex justify-center">
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={jobsQuery.isFetchingNextPage}
                  onClick={() => void jobsQuery.fetchNextPage()}
                >
                  {jobsQuery.isFetchingNextPage ? '加载中…' : '加载更多'}
                </Button>
              </div>
            ) : null}
          </>
        )
      ) : null}
    </div>
  )
}

function JobRow({ job }: { job: Job }) {
  const statusMeta = JOB_STATUS_META[job.status]
  const active = isJobActive(job.status)
  return (
    <li>
      <Link
        to="/jobs/$jobId"
        params={{ jobId: job.id }}
        className="group block px-4 py-3 hover:bg-surface-sunken"
      >
        <span className="flex items-center gap-3">
          <span className="min-w-0 flex-1">
            <span className="flex flex-wrap items-center gap-2">
              <span className="truncate font-medium text-text group-hover:text-accent">
                {JOB_KIND_LABEL[job.kind]}
              </span>
              <Badge variant={statusMeta.variant}>{statusMeta.label}</Badge>
            </span>
            <span className="mt-0.5 block truncate text-sm text-text-muted">
              {active ? job.stage : (job.error?.detail ?? job.message ?? job.stage)}
            </span>
          </span>
          <span className="shrink-0 text-xs text-text-subtle">
            {formatDateTime(job.updated_at)}
          </span>
          <ChevronRight
            aria-hidden="true"
            className="h-4 w-4 shrink-0 text-text-subtle group-hover:text-accent"
          />
        </span>
        {active ? (
          <span
            role="progressbar"
            aria-valuenow={job.progress}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label={`${JOB_KIND_LABEL[job.kind]}进度`}
            className="mt-2 block h-1.5 w-full rounded-full bg-surface-sunken"
          >
            <span
              className="block h-full rounded-full bg-info transition-[width] duration-180"
              style={{ width: `${job.progress}%` }}
            />
          </span>
        ) : null}
      </Link>
    </li>
  )
}
