import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { TrendingUp } from 'lucide-react'

import { listCourses } from '@/api/generated/endpoints/courses/courses'
import { getCourseProgress } from '@/api/generated/endpoints/progress/progress'
import type { Course } from '@/api/generated/model'
import { queryKeys } from '@/api/query-client'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { PageHeader } from '@/components/ui/page-header'
import { SkeletonRows } from '@/components/ui/skeleton'
import { MasteryTrend } from '@/features/progress/mastery-trend'

/**
 * 学习进度：按课程展示掌握度与趋势。掌握度与趋势点均由后端计算
 * （formula_version），前端只格式化展示，不重算。
 */
export const Route = createFileRoute('/_shell/progress')({
  component: ProgressPage,
})

function ProgressPage() {
  const coursesQuery = useQuery({
    queryKey: queryKeys.courses({ limit: 20, archived: false }),
    queryFn: () => listCourses({ limit: 20, archived: false }),
  })

  return (
    <div className="space-y-5">
      <PageHeader title="进度" description="按课程查看掌握度、完成的测验与活跃弱点。" />

      {coursesQuery.isPending ? (
        <div role="status" aria-busy="true" aria-label="进度加载中">
          <SkeletonRows rows={2} />
        </div>
      ) : null}

      {coursesQuery.isError ? (
        <ErrorState
          title="进度加载失败"
          error={coursesQuery.error}
          onRetry={() => void coursesQuery.refetch()}
        />
      ) : null}

      {coursesQuery.isSuccess ? (
        coursesQuery.data.items.length === 0 ? (
          <EmptyState
            icon={TrendingUp}
            title="还没有进度数据"
            description="创建课程并完成测验后，这里会显示你的掌握度趋势。"
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
          <ul className="grid gap-4 sm:grid-cols-2">
            {coursesQuery.data.items.map((course) => (
              <li key={course.id}>
                <CourseProgressCard course={course} />
              </li>
            ))}
          </ul>
        )
      ) : null}
    </div>
  )
}

function CourseProgressCard({ course }: { course: Course }) {
  const progressQuery = useQuery({
    queryKey: queryKeys.progress(course.id),
    queryFn: () => getCourseProgress(course.id),
  })

  return (
    <section
      aria-label={`${course.title} 进度`}
      className="rounded-md border border-border bg-surface p-4"
    >
      <h2 className="truncate text-sm font-medium">
        <Link
          to="/courses/$courseId"
          params={{ courseId: course.id }}
          className="rounded-sm text-text hover:text-accent"
        >
          {course.title}
        </Link>
      </h2>
      {progressQuery.isPending ? (
        <div
          role="status"
          aria-busy="true"
          aria-label={`${course.title} 进度加载中`}
          className="mt-3 space-y-2"
        >
          <div className="h-2 animate-pulse rounded-full bg-surface-sunken" />
          <div className="h-3 w-1/2 animate-pulse rounded bg-surface-sunken" />
        </div>
      ) : null}
      {progressQuery.isError ? (
        <p role="alert" className="mt-3 text-sm text-text-muted">
          进度加载失败。
          <button
            type="button"
            onClick={() => void progressQuery.refetch()}
            className="ml-1 rounded-sm font-medium text-accent hover:underline"
          >
            重试
          </button>
        </p>
      ) : null}
      {progressQuery.isSuccess ? (
        <div className="mt-3">
          {typeof progressQuery.data.mastery === 'number' ? (
            <div
              role="progressbar"
              aria-valuenow={Math.round(progressQuery.data.mastery * 100)}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-label={`${course.title} 掌握度`}
              className="h-2 w-full rounded-full bg-surface-sunken"
            >
              <div
                className="h-full rounded-full bg-accent"
                style={{ width: `${Math.round(progressQuery.data.mastery * 100)}%` }}
              />
            </div>
          ) : (
            <p className="text-sm text-text-muted">暂无掌握度数据，完成一次测验后开始统计。</p>
          )}
          <dl className="mt-3 flex gap-6 text-sm">
            <div>
              <dt className="text-xs text-text-subtle">掌握度</dt>
              <dd className="font-medium text-text">
                {typeof progressQuery.data.mastery === 'number'
                  ? `${Math.round(progressQuery.data.mastery * 100)}%`
                  : '—'}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-text-subtle">已完成测验</dt>
              <dd className="font-medium text-text">{progressQuery.data.completed_attempts}</dd>
            </div>
            <div>
              <dt className="text-xs text-text-subtle">活跃弱点</dt>
              <dd className="font-medium text-text">{progressQuery.data.active_weaknesses}</dd>
            </div>
          </dl>
          <div className="mt-3">
            <MasteryTrend trend={progressQuery.data.trend} label={course.title} />
          </div>
        </div>
      ) : null}
    </section>
  )
}
