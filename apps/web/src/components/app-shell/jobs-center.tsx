import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Content, Portal, Root, Trigger } from '@radix-ui/react-popover'
import { ListTodo } from 'lucide-react'
import { useState, useSyncExternalStore } from 'react'

import { listJobs } from '@/api/generated/endpoints/jobs/jobs'
import type { Job } from '@/api/generated/model'
import { getJobStreamStatus, subscribeJobStreamStatus } from '@/api/job-stream-status'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { buttonClasses } from '@/components/ui/button-styles'
import { SkeletonRows } from '@/components/ui/skeleton'
import { cn } from '@/lib/cn'
import { formatDateTime } from '@/lib/format'
import { isJobActive, JOB_KIND_LABEL, JOB_STATUS_META } from '@/lib/status'

/**
 * 任务中心：全局常驻顶栏，角标显示进行中的任务数（active=true 查询）。
 * 数据新鲜度由全局 SSE 驱动（事件就地合并 + 终态精准失效 + 断线 REST
 * 对账），本组件不轮询。面板展示最近任务，页脚进 /jobs。
 */
export function JobsCenter() {
  const [open, setOpen] = useState(false)
  const streamState = useSyncExternalStore(
    subscribeJobStreamStatus,
    () => getJobStreamStatus().state,
  )

  const activeQuery = useQuery({
    queryKey: queryKeys.jobs({ active: true }),
    queryFn: () => listJobs({ active: true }),
  })
  const recentQuery = useQuery({
    queryKey: queryKeys.jobs({ limit: 5 }),
    queryFn: () => listJobs({ limit: 5 }),
  })

  const activeCount = activeQuery.data?.items.length ?? 0
  const triggerLabel =
    activeCount > 0 ? `任务中心，${activeCount} 个任务进行中` : '任务中心，暂无进行中的任务'

  return (
    <Root open={open} onOpenChange={setOpen}>
      <Trigger asChild>
        <button
          type="button"
          aria-label={triggerLabel}
          className={cn(buttonClasses({ variant: 'ghost', size: 'sm' }), 'relative')}
        >
          <ListTodo aria-hidden="true" className="h-4 w-4" />
          <span className="hidden sm:inline">任务</span>
          {activeCount > 0 ? (
            <span
              aria-hidden="true"
              className="flex h-4 min-w-4 items-center justify-center rounded-full bg-info px-1 text-[10px] font-semibold text-accent-contrast"
            >
              {activeCount > 9 ? '9+' : activeCount}
            </span>
          ) : null}
        </button>
      </Trigger>
      <Portal>
        <Content
          align="end"
          sideOffset={6}
          className="z-50 w-80 rounded-md border border-border bg-surface shadow-dialog"
        >
          <div className="flex items-center justify-between border-b border-border px-3 py-2">
            <h2 className="text-sm font-semibold text-text">最近任务</h2>
            <Link
              to="/jobs"
              onClick={() => setOpen(false)}
              className="rounded-sm text-sm text-accent hover:underline"
            >
              查看全部
            </Link>
          </div>
          {streamState === 'reconnecting' ? (
            <p className="border-b border-border bg-surface-sunken px-3 py-1.5 text-xs text-text-muted">
              任务事件连接中断，正在重连并对账…
            </p>
          ) : null}
          <JobsCenterBody
            isPending={recentQuery.isPending}
            isError={recentQuery.isError}
            jobs={recentQuery.data?.items}
            onRetry={() => void recentQuery.refetch()}
            onNavigate={() => setOpen(false)}
          />
        </Content>
      </Portal>
    </Root>
  )
}

function JobsCenterBody({
  isPending,
  isError,
  jobs,
  onRetry,
  onNavigate,
}: {
  isPending: boolean
  isError: boolean
  jobs?: Job[]
  onRetry: () => void
  onNavigate: () => void
}) {
  if (isPending) {
    return (
      <div role="status" aria-busy="true" aria-label="任务加载中" className="p-2">
        <SkeletonRows rows={2} />
      </div>
    )
  }
  if (isError) {
    return (
      <div role="alert" className="px-3 py-4 text-sm">
        <p className="text-text-muted">任务列表加载失败。</p>
        <button
          type="button"
          onClick={onRetry}
          className="mt-1 rounded-sm font-medium text-accent hover:underline"
        >
          重试
        </button>
      </div>
    )
  }
  if (!jobs || jobs.length === 0) {
    return (
      <p className="px-3 py-4 text-sm text-text-muted">
        暂无任务。导入课时或生成测验后会出现在这里。
      </p>
    )
  }
  return (
    <ul className="max-h-80 divide-y divide-border overflow-y-auto">
      {jobs.map((job) => (
        <li key={job.id}>
          <Link
            to="/jobs/$jobId"
            params={{ jobId: job.id }}
            onClick={onNavigate}
            className="block px-3 py-2 hover:bg-surface-sunken"
          >
            <span className="flex items-center justify-between gap-2">
              <span className="truncate text-sm font-medium text-text">
                {JOB_KIND_LABEL[job.kind] ?? job.kind}
              </span>
              <Badge variant={JOB_STATUS_META[job.status].variant}>
                {JOB_STATUS_META[job.status].label}
              </Badge>
            </span>
            <span className="mt-0.5 flex items-center justify-between gap-2 text-xs text-text-subtle">
              <span className="truncate">
                {isJobActive(job.status) ? job.stage : (job.message ?? '')}
              </span>
              <span className="shrink-0">{formatDateTime(job.updated_at)}</span>
            </span>
          </Link>
        </li>
      ))}
    </ul>
  )
}
