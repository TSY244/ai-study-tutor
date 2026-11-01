import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { useEffect } from 'react'

import { getLesson } from '@/api/generated/endpoints/lessons/lessons'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { ErrorState } from '@/components/ui/error-state'
import { Skeleton } from '@/components/ui/skeleton'
import { recordRecentLearning } from '@/features/learning/recent-learning'
import { LessonActions } from '@/features/lessons/lesson-actions'
import { ImportPanel } from '@/features/materials/import-panel'
import { cn } from '@/lib/cn'
import { LESSON_STATUS_META } from '@/lib/status'

const SUB_VIEWS = [
  { to: '/courses/$courseId/lessons/$lessonId/notes', label: '笔记' },
  { to: '/courses/$courseId/lessons/$lessonId/raw', label: '原文' },
  { to: '/courses/$courseId/lessons/$lessonId/outline', label: '大纲' },
  { to: '/courses/$courseId/lessons/$lessonId/quizzes', label: '测验' },
  { to: '/courses/$courseId/lessons/$lessonId/attempts', label: '作答' },
  { to: '/courses/$courseId/lessons/$lessonId/summary', label: '总结' },
] as const

/**
 * 课时布局：课时标题 + 状态 + 子视图导航（笔记/大纲/测验/作答/总结）。
 * 子视图是路由跳转而非同页面板，因此用语义化 nav + aria-current，
 * 不用 Tabs。crumb '$lesson' 由面包屑替换为课时标题。
 */
export const Route = createFileRoute('/_shell/courses/$courseId/lessons/$lessonId')({
  staticData: { crumb: '$lesson' },
  component: LessonLayout,
})

function LessonLayout() {
  const { courseId, lessonId } = Route.useParams()
  const lessonQuery = useQuery({
    queryKey: queryKeys.lesson(lessonId),
    queryFn: () => getLesson(lessonId),
  })

  // 打开课时工作区即视为一次学习：写入「继续学习」最近记录
  const lessonTitle = lessonQuery.isSuccess ? lessonQuery.data.title : null
  useEffect(() => {
    if (lessonTitle !== null) {
      recordRecentLearning({ courseId, lessonId, lessonTitle })
    }
  }, [courseId, lessonId, lessonTitle])

  return (
    <div className="space-y-4">
      {lessonQuery.isPending ? (
        <div role="status" aria-busy="true" aria-label="课时加载中" className="space-y-2">
          <Skeleton className="h-6 w-64" />
          <Skeleton className="h-4 w-full max-w-lg" />
        </div>
      ) : null}

      {lessonQuery.isError ? (
        <ErrorState
          title="课时加载失败"
          error={lessonQuery.error}
          onRetry={() => void lessonQuery.refetch()}
        />
      ) : null}

      {lessonQuery.isSuccess ? (
        <>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
              <h1 className="text-lg font-semibold text-text">{lessonQuery.data.title}</h1>
              <Badge variant={LESSON_STATUS_META[lessonQuery.data.status].variant}>
                {LESSON_STATUS_META[lessonQuery.data.status].label}
              </Badge>
              {typeof lessonQuery.data.mastery === 'number' ? (
                <span className="text-sm text-text-muted">
                  掌握度 {Math.round(lessonQuery.data.mastery * 100)}%
                </span>
              ) : null}
            </div>
            <LessonActions lesson={lessonQuery.data} />
          </div>
        </>
      ) : null}

      <nav aria-label="课时子视图" className="-mb-px overflow-x-auto border-b border-border">
        <ul className="flex items-center gap-1 text-sm">
          {SUB_VIEWS.map((view) => (
            <li key={view.to} className="shrink-0">
              <Link
                to={view.to}
                params={{ courseId, lessonId }}
                activeOptions={{ exact: true }}
                activeProps={{
                  'aria-current': 'page',
                  className: 'border-accent font-medium text-accent-strong',
                }}
                inactiveProps={{
                  className: 'border-transparent text-text-muted hover:text-text',
                }}
                className={cn('block border-b-2 px-3 py-2 transition-colors duration-120')}
              >
                {view.label}
              </Link>
            </li>
          ))}
        </ul>
      </nav>

      {/* 导入表单与进度随当前栏目展示，避免在标签栏上下重复显示任务。 */}
      {lessonQuery.isSuccess ? <ImportPanel lesson={lessonQuery.data} /> : null}

      <Outlet />
    </div>
  )
}
