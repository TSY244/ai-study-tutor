import { DeleteContentButton } from '@/features/content/delete-content-button'
import { useInfiniteQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'

import { listLessonAttempts } from '@/api/generated/endpoints/attempts/attempts'
import type { AttemptSummary } from '@/api/generated/model'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { SkeletonText } from '@/components/ui/skeleton'
import { ResourceJobs } from '@/features/jobs/resource-jobs'
import { useActiveResourceJobs } from '@/features/jobs/use-active-resource-jobs'
import { formatDateTime } from '@/lib/format'
import { ATTEMPT_STATUS_META } from '@/lib/status'

export const Route = createFileRoute('/_shell/courses/$courseId/lessons/$lessonId/attempts')({
  staticData: { crumb: '作答' },
  component: LessonAttemptsPage,
})

/**
 * 课时作答历史：游标分页（started_at 倒序）。行进专注模式——已批阅
 * 看成绩与 AI 逐题反馈，未完成继续作答；新作答从测验页开始。
 * 批阅终态由全局任务事件流精准失效本列表（job-stream）。
 */
function LessonAttemptsPage() {
  const { courseId, lessonId } = Route.useParams()
  const jobsQuery = useActiveResourceJobs('attempt', undefined, [
    'attempt_grade',
    'wrong_answer_analyze',
  ])
  const attemptsQuery = useInfiniteQuery({
    queryKey: queryKeys.lessonAttempts(lessonId),
    queryFn: ({ pageParam }) => listLessonAttempts(lessonId, { cursor: pageParam, limit: 20 }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) =>
      lastPage.page_info.has_more ? (lastPage.page_info.next_cursor ?? undefined) : undefined,
  })

  const header = (
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0 space-y-1">
        <h2 className="text-base font-semibold text-text">作答记录</h2>
        <p className="text-sm text-text-muted">
          这里会显示作答批阅进度；完成后可查看成绩与 AI 反馈。
        </p>
      </div>
      <Link
        to="/courses/$courseId/lessons/$lessonId/quizzes"
        params={{ courseId, lessonId }}
        className="rounded-md bg-accent px-3 py-1.5 text-sm font-medium text-accent-contrast hover:bg-accent-strong"
      >
        前往测验
      </Link>
    </div>
  )

  if (attemptsQuery.isPending) {
    return (
      <div className="space-y-4">
        {header}
        <div role="status" aria-busy="true" aria-label="作答记录加载中" className="space-y-3">
          <SkeletonText lines={2} />
          <SkeletonText lines={2} />
        </div>
      </div>
    )
  }
  if (attemptsQuery.isError) {
    return (
      <div className="space-y-4">
        {header}
        <ErrorState
          title="作答记录加载失败"
          error={attemptsQuery.error}
          onRetry={() => void attemptsQuery.refetch()}
        />
      </div>
    )
  }

  const attempts = attemptsQuery.data.pages.flatMap((page) => page.items)

  if (attempts.length === 0) {
    return (
      <div className="space-y-4">
        {header}
        <EmptyState
          title="本课时还没有作答记录"
          description="从测验页开始一次作答；提交后成绩与 AI 反馈会出现在这里。"
        />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {header}
      <ResourceJobs
        query={{
          ...jobsQuery,
          jobs: jobsQuery.jobs.filter((job) =>
            attempts.some((attempt) => attempt.id === job.aggregate_id),
          ),
        }}
        title="作答正在处理"
      />
      <ul className="divide-y divide-border rounded-md border border-border bg-surface">
        {attempts.map((attempt) => (
          <AttemptRow key={attempt.id} attempt={attempt} courseId={courseId} lessonId={lessonId} />
        ))}
      </ul>
      {attemptsQuery.hasNextPage ? (
        <div>
          <Button
            variant="secondary"
            size="sm"
            disabled={attemptsQuery.isFetchingNextPage}
            onClick={() => void attemptsQuery.fetchNextPage()}
          >
            {attemptsQuery.isFetchingNextPage ? '加载中…' : '加载更多'}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

function AttemptRow({
  attempt,
  courseId,
  lessonId,
}: {
  attempt: AttemptSummary
  courseId: string
  lessonId: string
}) {
  const meta = ATTEMPT_STATUS_META[attempt.status]
  return (
    <li className="flex items-center pr-3">
      <Link
        to="/attempts/$attemptId"
        params={{ attemptId: attempt.id }}
        className="group flex min-w-0 flex-1 items-center gap-3 px-4 py-3 hover:bg-surface-sunken"
      >
        <span className="min-w-0 flex-1">
          <span className="flex flex-wrap items-center gap-2">
            <span className="truncate text-sm font-medium text-text group-hover:text-accent">
              {attempt.quiz_title}
            </span>
            <Badge variant={meta.variant}>{meta.label}</Badge>
          </span>
          <span className="mt-0.5 block truncate text-xs text-text-subtle">
            {attempt.status === 'graded' && typeof attempt.final_score === 'number'
              ? `成绩 ${attempt.final_score}${typeof attempt.max_score === 'number' ? ` / ${attempt.max_score}` : ''} · `
              : ''}
            {attempt.submitted_at
              ? `提交于 ${formatDateTime(attempt.submitted_at)}`
              : `开始于 ${formatDateTime(attempt.started_at)}`}
          </span>
        </span>
        <ChevronRight
          aria-hidden="true"
          className="h-4 w-4 shrink-0 text-text-subtle group-hover:text-accent"
        />
      </Link>
      <DeleteContentButton
        kind="attempt"
        id={attempt.id}
        name={`${attempt.quiz_title} · ${formatDateTime(attempt.started_at)}`}
        courseId={courseId}
        lessonId={lessonId}
      />
    </li>
  )
}
