import { useInfiniteQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { BookOpen, ChevronRight, Plus } from 'lucide-react'

import { listCourses } from '@/api/generated/endpoints/courses/courses'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { buttonClasses } from '@/components/ui/button-styles'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { PageHeader } from '@/components/ui/page-header'
import { SkeletonRows } from '@/components/ui/skeleton'
import { formatDateTime } from '@/lib/format'

const PAGE_SIZE = 20

/**
 * 课程列表：游标分页（page_info.next_cursor），“加载更多”追加；
 * 空/加载/错误三态齐备。
 */
export const Route = createFileRoute('/_shell/courses/')({
  component: CoursesPage,
})

function CoursesPage() {
  const coursesQuery = useInfiniteQuery({
    queryKey: queryKeys.courses({ view: 'paged', limit: PAGE_SIZE }),
    queryFn: ({ pageParam }) => listCourses({ cursor: pageParam || undefined, limit: PAGE_SIZE }),
    initialPageParam: '',
    getNextPageParam: (lastPage) =>
      lastPage.page_info.has_more ? (lastPage.page_info.next_cursor ?? undefined) : undefined,
  })

  const courses = coursesQuery.data?.pages.flatMap((page) => page.items) ?? []

  return (
    <div className="space-y-5">
      <PageHeader
        title="课程"
        description="课程是学习的顶层容器，课时归属于课程。"
        actions={
          <Link to="/courses/new" className={buttonClasses({ size: 'sm' })}>
            <Plus aria-hidden="true" className="h-4 w-4" />
            新建课程
          </Link>
        }
      />

      {coursesQuery.isPending ? (
        <div role="status" aria-busy="true" aria-label="课程列表加载中">
          <SkeletonRows rows={4} />
        </div>
      ) : null}

      {coursesQuery.isError ? (
        <ErrorState
          title="课程列表加载失败"
          error={coursesQuery.error}
          onRetry={() => void coursesQuery.refetch()}
        />
      ) : null}

      {coursesQuery.isSuccess ? (
        courses.length === 0 ? (
          <EmptyState
            icon={BookOpen}
            title="还没有课程"
            description="创建第一门课程，然后通过文本、链接或文件导入课时。"
            action={
              <Link to="/courses/new" className={buttonClasses({ size: 'sm' })}>
                <Plus aria-hidden="true" className="h-4 w-4" />
                新建课程
              </Link>
            }
          />
        ) : (
          <>
            <ul className="divide-y divide-border rounded-md border border-border bg-surface">
              {courses.map((course) => (
                <li key={course.id}>
                  <Link
                    to="/courses/$courseId"
                    params={{ courseId: course.id }}
                    className="group flex items-center gap-4 px-4 py-3 hover:bg-surface-sunken"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="flex items-center gap-2">
                        <span className="truncate font-medium text-text group-hover:text-accent">
                          {course.title}
                        </span>
                        {course.archived ? <Badge>已归档</Badge> : null}
                      </span>
                      {course.description ? (
                        <span className="mt-0.5 line-clamp-2 block text-sm text-text-muted">
                          {course.description}
                        </span>
                      ) : null}
                    </span>
                    <span className="shrink-0 text-right text-xs text-text-subtle">
                      <span className="block">{course.lesson_count} 个课时</span>
                      <span className="mt-0.5 block">
                        更新于 {formatDateTime(course.updated_at)}
                      </span>
                    </span>
                    <ChevronRight
                      aria-hidden="true"
                      className="h-4 w-4 shrink-0 text-text-subtle group-hover:text-accent"
                    />
                  </Link>
                </li>
              ))}
            </ul>
            {coursesQuery.hasNextPage ? (
              <div className="flex justify-center">
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={coursesQuery.isFetchingNextPage}
                  onClick={() => void coursesQuery.fetchNextPage()}
                >
                  {coursesQuery.isFetchingNextPage ? '加载中…' : '加载更多'}
                </Button>
              </div>
            ) : null}
          </>
        )
      ) : null}
    </div>
  )
}
