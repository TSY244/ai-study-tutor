import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'

import { getAttempt } from '@/api/generated/endpoints/attempts/attempts'
import { getLesson } from '@/api/generated/endpoints/lessons/lessons'
import { getQuiz } from '@/api/generated/endpoints/quizzes/quizzes'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { ErrorState } from '@/components/ui/error-state'
import { PathLink } from '@/components/ui/path-link'
import { Skeleton, SkeletonText } from '@/components/ui/skeleton'
import { AttemptResult } from '@/features/attempts/attempt-result'
import { AttemptTaking } from '@/features/attempts/attempt-taking'
import { ATTEMPT_STATUS_META } from '@/lib/status'

/**
 * 作答专注模式：不使用 App Shell——无全局导航与任务中心入口，
 * 只保留退出链路、测验标题与作答状态，减少考试中的干扰。
 * - in_progress：作答视图（自动保存 + 刷新恢复 + 提交）。
 * - submitted/grading：结果视图等待批阅（全局 SSE 终态事件触发
 *   attempt 精准失效后 REST 重取，不轮询）。
 * - graded/grading_failed：结果与批阅反馈/重试入口。
 */
export const Route = createFileRoute('/attempts/$attemptId')({
  component: AttemptFocusPage,
})

function AttemptFocusPage() {
  const { attemptId } = Route.useParams()

  const attemptQuery = useQuery({
    queryKey: queryKeys.attempt(attemptId),
    queryFn: () => getAttempt(attemptId),
  })
  const quizId = attemptQuery.data?.quiz_id
  const quizQuery = useQuery({
    queryKey: queryKeys.quiz(quizId ?? ''),
    queryFn: () => getQuiz(quizId as string),
    enabled: Boolean(quizId),
  })
  const lessonId = quizQuery.data?.lesson_id
  const lessonQuery = useQuery({
    queryKey: queryKeys.lesson(lessonId ?? ''),
    queryFn: () => getLesson(lessonId as string),
    enabled: Boolean(lessonId),
  })

  // 退出链路：优先回到所属课时的作答页；解析失败时退回学习总览。
  const exitTo = lessonQuery.isSuccess
    ? `/courses/${lessonQuery.data.course_id}/lessons/${lessonQuery.data.id}/attempts`
    : '/dashboard'

  return (
    <div className="flex min-h-dvh flex-col bg-canvas">
      <a href="#main-content" className="skip-link">
        跳到主要内容
      </a>
      <header className="border-b border-border bg-surface">
        <div className="mx-auto flex h-12 max-w-3xl items-center gap-3 px-4">
          <PathLink
            to={exitTo}
            className="flex shrink-0 items-center gap-1 rounded-md px-2 py-1.5 text-sm text-text-muted hover:bg-surface-sunken hover:text-text"
          >
            <ArrowLeft aria-hidden="true" className="h-4 w-4" />
            退出作答
          </PathLink>
          <div className="min-w-0 flex-1 text-center">
            {quizQuery.isSuccess ? (
              <h1 className="truncate text-sm font-medium text-text">{quizQuery.data.title}</h1>
            ) : (
              <Skeleton className="mx-auto h-4 w-40" />
            )}
          </div>
          <div className="flex w-24 shrink-0 justify-end">
            {attemptQuery.isSuccess ? (
              <Badge variant={ATTEMPT_STATUS_META[attemptQuery.data.status].variant}>
                {ATTEMPT_STATUS_META[attemptQuery.data.status].label}
              </Badge>
            ) : null}
          </div>
        </div>
      </header>

      <main
        id="main-content"
        tabIndex={-1}
        className="mx-auto w-full max-w-3xl flex-1 px-4 py-6 outline-none"
      >
        {attemptQuery.isPending ? (
          <div role="status" aria-busy="true" aria-label="作答加载中" className="space-y-4">
            <SkeletonText lines={3} />
            <Skeleton className="h-40 w-full" />
          </div>
        ) : null}

        {attemptQuery.isError ? (
          <ErrorState
            title="作答加载失败"
            error={attemptQuery.error}
            onRetry={() => void attemptQuery.refetch()}
          />
        ) : null}

        {attemptQuery.isSuccess ? (
          attemptQuery.data.status === 'in_progress' ? (
            <AttemptTaking attempt={attemptQuery.data} />
          ) : (
            <AttemptResult attempt={attemptQuery.data} />
          )
        ) : null}
      </main>
    </div>
  )
}
