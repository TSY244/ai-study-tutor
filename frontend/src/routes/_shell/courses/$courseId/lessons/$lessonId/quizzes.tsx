import { useInfiniteQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { listQuizzes } from '@/api/generated/endpoints/quizzes/quizzes'
import type { QuizList } from '@/api/generated/model'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { SkeletonText } from '@/components/ui/skeleton'
import { StartAttemptButton } from '@/features/attempts/start-attempt-button'
import { GenerateQuizButton } from '@/features/quizzes/generate-quiz-button'
import { formatDateTime } from '@/lib/format'

export const Route = createFileRoute('/_shell/courses/$courseId/lessons/$lessonId/quizzes')({
  staticData: { crumb: '测验' },
  component: LessonQuizzesPage,
})

const QUIZ_SOURCE_LABEL: Record<QuizList['items'][number]['source'], string> = {
  regular: '随堂测验',
  retest: '复习测验',
  assembled: '组卷',
}

/**
 * 课时测验列表：游标分页；每份测验可直接「开始作答」（创建 Attempt
 * 并跳转专注模式）。历史作答见同课时「作答」页（listLessonAttempts）。
 */
function LessonQuizzesPage() {
  const { courseId, lessonId } = Route.useParams()
  const quizzesQuery = useInfiniteQuery({
    queryKey: queryKeys.quizzes(lessonId),
    queryFn: ({ pageParam }) => listQuizzes(lessonId, { cursor: pageParam, limit: 20 }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) =>
      lastPage.page_info.has_more ? (lastPage.page_info.next_cursor ?? undefined) : undefined,
  })

  if (quizzesQuery.isPending) {
    return (
      <div role="status" aria-busy="true" aria-label="测验列表加载中" className="space-y-3">
        <SkeletonText lines={2} />
        <SkeletonText lines={2} />
      </div>
    )
  }
  if (quizzesQuery.isError) {
    return (
      <ErrorState
        title="测验列表加载失败"
        error={quizzesQuery.error}
        onRetry={() => void quizzesQuery.refetch()}
      />
    )
  }

  const quizzes = quizzesQuery.data.pages.flatMap((page) => page.items)
  const header = (
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0 space-y-1">
        <h2 className="text-base font-semibold text-text">测验</h2>
        <p className="text-sm text-text-muted">
          常规测验会由后端根据课时内容、历史成绩和薄弱点策略生成。
        </p>
      </div>
      <GenerateQuizButton courseId={courseId} lessonId={lessonId} />
    </div>
  )

  if (quizzes.length === 0) {
    return (
      <div className="space-y-4">
        {header}
        <EmptyState title="本课时还没有测验" description="课时内容就绪后可以生成测验。" />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {header}
      <ul className="space-y-3">
        {quizzes.map((quiz) => (
          <li
            key={quiz.id}
            className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-surface p-4"
          >
            <div className="min-w-0 space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-sm font-medium text-text">{quiz.title}</h2>
                <Badge variant="neutral">{QUIZ_SOURCE_LABEL[quiz.source]}</Badge>
              </div>
              <p className="text-xs text-text-subtle">
                {quiz.question_count} 题 · 创建于 {formatDateTime(quiz.created_at)}
              </p>
            </div>
            <StartAttemptButton quiz={quiz} />
          </li>
        ))}
      </ul>
      {quizzesQuery.hasNextPage ? (
        <div>
          <Button
            variant="secondary"
            size="sm"
            disabled={quizzesQuery.isFetchingNextPage}
            onClick={() => void quizzesQuery.fetchNextPage()}
          >
            {quizzesQuery.isFetchingNextPage ? '加载中…' : '加载更多'}
          </Button>
        </div>
      ) : null}
    </div>
  )
}
