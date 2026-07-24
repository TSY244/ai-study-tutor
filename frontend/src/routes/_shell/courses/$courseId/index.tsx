import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { BookOpen, ChevronRight, Plus } from 'lucide-react'

import { getCourse } from '@/api/generated/endpoints/courses/courses'
import { listLessons } from '@/api/generated/endpoints/lessons/lessons'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { buttonClasses } from '@/components/ui/button-styles'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { PageHeader } from '@/components/ui/page-header'
import { Skeleton, SkeletonRows } from '@/components/ui/skeleton'
import { CourseActions } from '@/features/courses/course-actions'
import { formatDateTime } from '@/lib/format'
import { LESSON_STATUS_META } from '@/lib/status'

const PAGE_SIZE = 50

/**
 * 课程详情：课程信息 + 操作（编辑/归档/删除）+ 课时列表（游标分页）。
 * 新建课时进入独立表单页；课时导入在课时页完成。
 */
export const Route = createFileRoute('/_shell/courses/$courseId/')({
  component: CourseDetailPage,
})

function CourseDetailPage() {
  const { courseId } = Route.useParams()

  const courseQuery = useQuery({
    queryKey: queryKeys.course(courseId),
    queryFn: () => getCourse(courseId),
  })
  const lessonsQuery = useInfiniteQuery({
    queryKey: [...queryKeys.lessons(courseId), { limit: PAGE_SIZE }] as const,
    queryFn: ({ pageParam }) =>
      listLessons(courseId, { cursor: pageParam || undefined, limit: PAGE_SIZE }),
    initialPageParam: '',
    getNextPageParam: (lastPage) =>
      lastPage.page_info.has_more ? (lastPage.page_info.next_cursor ?? undefined) : undefined,
  })

  if (courseQuery.isPending) {
    return (
      <div role="status" aria-busy="true" aria-label="课程加载中" className="space-y-5">
        <div className="space-y-2">
          <Skeleton className="h-6 w-56" />
          <Skeleton className="h-4 w-full max-w-lg" />
        </div>
        <SkeletonRows rows={3} />
      </div>
    )
  }

  if (courseQuery.isError) {
    return (
      <ErrorState
        title="课程加载失败"
        error={courseQuery.error}
        onRetry={() => void courseQuery.refetch()}
      />
    )
  }

  const course = courseQuery.data
  const lessons = lessonsQuery.data?.pages.flatMap((page) => page.items) ?? []

  return (
    <div className="space-y-5">
      <PageHeader
        title={course.title}
        description={course.description || undefined}
        actions={
          <div className="flex items-center gap-2">
            {course.archived ? <Badge>已归档</Badge> : null}
            <CourseActions course={course} />
          </div>
        }
      />

      <section aria-labelledby="lessons-heading">
        <div className="flex items-center justify-between gap-3">
          <h2 id="lessons-heading" className="text-base font-medium text-text">
            课时
          </h2>
          <Link
            to="/courses/$courseId/lessons/new"
            params={{ courseId }}
            className={buttonClasses({ variant: 'secondary', size: 'sm' })}
          >
            <Plus aria-hidden="true" className="h-4 w-4" />
            新建课时
          </Link>
        </div>
        <div className="mt-3">
          {lessonsQuery.isPending ? (
            <div role="status" aria-busy="true" aria-label="课时加载中">
              <SkeletonRows rows={3} />
            </div>
          ) : null}

          {lessonsQuery.isError ? (
            <ErrorState
              title="课时列表加载失败"
              error={lessonsQuery.error}
              onRetry={() => void lessonsQuery.refetch()}
            />
          ) : null}

          {lessonsQuery.isSuccess ? (
            lessons.length === 0 ? (
              <EmptyState
                icon={BookOpen}
                title="这门课程还没有课时"
                description="创建课时后，通过文本、链接或文件导入素材，系统会自动生成笔记与教纲。"
                action={
                  <Link
                    to="/courses/$courseId/lessons/new"
                    params={{ courseId }}
                    className={buttonClasses({ size: 'sm' })}
                  >
                    <Plus aria-hidden="true" className="h-4 w-4" />
                    新建课时
                  </Link>
                }
              />
            ) : (
              <>
                <ul className="divide-y divide-border rounded-md border border-border bg-surface">
                  {lessons.map((lesson) => (
                    <li key={lesson.id}>
                      <Link
                        to="/courses/$courseId/lessons/$lessonId"
                        params={{ courseId, lessonId: lesson.id }}
                        className="group flex items-center gap-4 px-4 py-3 hover:bg-surface-sunken"
                      >
                        <span className="w-14 shrink-0 text-sm text-text-subtle">
                          第 {lesson.ordinal} 课
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="flex flex-wrap items-center gap-2">
                            <span className="truncate font-medium text-text group-hover:text-accent">
                              {lesson.title}
                            </span>
                            <Badge variant={LESSON_STATUS_META[lesson.status].variant}>
                              {LESSON_STATUS_META[lesson.status].label}
                            </Badge>
                          </span>
                          {lesson.description ? (
                            <span className="mt-0.5 line-clamp-1 block text-sm text-text-muted">
                              {lesson.description}
                            </span>
                          ) : null}
                        </span>
                        <span className="shrink-0 text-xs text-text-subtle">
                          {typeof lesson.mastery === 'number'
                            ? `掌握度 ${Math.round(lesson.mastery * 100)}%`
                            : ''}
                        </span>
                        <ChevronRight
                          aria-hidden="true"
                          className="h-4 w-4 shrink-0 text-text-subtle group-hover:text-accent"
                        />
                      </Link>
                    </li>
                  ))}
                </ul>
                {lessonsQuery.hasNextPage ? (
                  <div className="mt-3 flex justify-center">
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={lessonsQuery.isFetchingNextPage}
                      onClick={() => void lessonsQuery.fetchNextPage()}
                    >
                      {lessonsQuery.isFetchingNextPage ? '加载中…' : '加载更多'}
                    </Button>
                  </div>
                ) : null}
              </>
            )
          ) : null}
        </div>
      </section>

      <p className="text-xs text-text-subtle">
        更新于 {formatDateTime(course.updated_at)} · 版本 {course.version}
      </p>
    </div>
  )
}
