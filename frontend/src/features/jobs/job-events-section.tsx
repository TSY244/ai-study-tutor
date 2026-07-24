import { useInfiniteQuery } from '@tanstack/react-query'

import { listJobEvents } from '@/api/generated/endpoints/jobs/jobs'
import type { JobEvent, JobEventEventType } from '@/api/generated/model'
import { queryKeys } from '@/api/query-client'
import { ErrorState } from '@/components/ui/error-state'
import { SkeletonRows } from '@/components/ui/skeleton'
import { formatDateTime } from '@/lib/format'

/** 审计事件类型 → 文案（与 JOB_STATUS_META 分属两个 Schema：JobEvent ≠ JobStreamData）。 */
const JOB_EVENT_LABEL: Record<JobEventEventType, string> = {
  'job.queued': '已排队',
  'job.started': '开始执行',
  'job.progress': '进度',
  'job.validating': '校验中',
  'job.cancel_requested': '请求取消',
  'job.succeeded': '已成功',
  'job.failed': '失败',
  'job.cancelled': '已取消',
}

const PAGE_SIZE = 20

/**
 * 任务事件时间线（REST 审计接口，游标分页追加）。
 * SSE 新事件到达时该 Key 被精准失效，仅在本页观察时重取第一页。
 */
export function JobEventsSection({ jobId }: { jobId: string }) {
  const eventsQuery = useInfiniteQuery({
    queryKey: queryKeys.jobEvents(jobId),
    queryFn: ({ pageParam }) =>
      listJobEvents(jobId, { cursor: pageParam || undefined, limit: PAGE_SIZE }),
    initialPageParam: '',
    getNextPageParam: (lastPage) =>
      lastPage.page_info.has_more ? (lastPage.page_info.next_cursor ?? undefined) : undefined,
  })

  return (
    <section aria-labelledby="job-events-heading">
      <h2 id="job-events-heading" className="text-base font-medium text-text">
        事件时间线
      </h2>
      <div className="mt-2 rounded-md border border-border bg-surface">
        {eventsQuery.isPending ? (
          <div role="status" aria-busy="true" aria-label="事件加载中" className="p-3">
            <SkeletonRows rows={3} />
          </div>
        ) : null}
        {eventsQuery.isError ? (
          <div className="p-3">
            <ErrorState
              title="事件加载失败"
              error={eventsQuery.error}
              onRetry={() => void eventsQuery.refetch()}
            />
          </div>
        ) : null}
        {eventsQuery.isSuccess ? (
          <JobEventList
            pages={eventsQuery.data.pages.map((page) => page.items)}
            hasMore={Boolean(eventsQuery.hasNextPage)}
            isFetchingMore={eventsQuery.isFetchingNextPage}
            onLoadMore={() => void eventsQuery.fetchNextPage()}
          />
        ) : null}
      </div>
    </section>
  )
}

function JobEventList({
  pages,
  hasMore,
  isFetchingMore,
  onLoadMore,
}: {
  pages: JobEvent[][]
  hasMore: boolean
  isFetchingMore: boolean
  onLoadMore: () => void
}) {
  const events = pages.flat()
  if (events.length === 0) {
    return <p className="px-4 py-3 text-sm text-text-muted">暂无事件记录。</p>
  }
  return (
    <>
      <ol className="divide-y divide-border">
        {events.map((event) => (
          <li key={event.id} className="flex items-baseline justify-between gap-3 px-4 py-2">
            <div className="min-w-0">
              <p className="text-sm text-text">
                <span className="font-medium">{JOB_EVENT_LABEL[event.event_type]}</span>
                <span className="text-text-muted">
                  {' '}
                  · {event.stage} · {event.progress}%
                </span>
              </p>
              {event.message ? (
                <p className="mt-0.5 truncate text-xs text-text-subtle">{event.message}</p>
              ) : null}
            </div>
            <time className="shrink-0 text-xs text-text-subtle">
              {formatDateTime(event.occurred_at)}
            </time>
          </li>
        ))}
      </ol>
      {hasMore ? (
        <div className="border-t border-border px-4 py-2">
          <button
            type="button"
            onClick={onLoadMore}
            disabled={isFetchingMore}
            className="rounded-sm text-sm font-medium text-accent hover:underline disabled:opacity-60"
          >
            {isFetchingMore ? '加载中…' : '加载更多'}
          </button>
        </div>
      ) : null}
    </>
  )
}
