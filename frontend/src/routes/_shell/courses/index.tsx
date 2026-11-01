import { DeleteContentButton } from '@/features/content/delete-content-button'
import { useInfiniteQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { BookOpen, ChevronRight, Plus } from 'lucide-react'
import { useState } from 'react'

import { listCourses } from '@/api/generated/endpoints/courses/courses'
import type { Course } from '@/api/generated/model'
import { presentProblem } from '@/api/problem-messages'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { buttonClasses } from '@/components/ui/button-styles'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { PageHeader } from '@/components/ui/page-header'
import { SelectionToolbar } from '@/components/ui/selection-toolbar'
import { SkeletonRows } from '@/components/ui/skeleton'
import { useBatchDeleteCourses } from '@/features/courses/use-course-mutations'
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

  // 多选删除：选择模式下整行变为 label + checkbox（不再跳转），
  // 删除走原子批量接口，409 resource_busy 等错误留在确认框内呈现。
  const [selecting, setSelecting] = useState(false)
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<string>>(new Set())
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const batchDelete = useBatchDeleteCourses()

  const toggleSelected = (courseId: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(courseId)) {
        next.delete(courseId)
      } else {
        next.add(courseId)
      }
      return next
    })
  }

  const exitSelecting = () => {
    setSelecting(false)
    setSelectedIds(new Set())
  }

  const confirmDelete = async () => {
    setDeleteError(null)
    try {
      await batchDelete.mutateAsync([...selectedIds])
      setDeleteOpen(false)
      exitSelecting()
    } catch (error) {
      const presentation = presentProblem(error)
      setDeleteError(`${presentation.title}：${presentation.message}`)
    }
  }

  return (
    <div className="space-y-5">
      <PageHeader
        title="课程"
        description="课程是学习的顶层容器，课时归属于课程。"
        actions={
          <div className="flex items-center gap-2">
            {coursesQuery.isSuccess && courses.length > 0 && !selecting ? (
              <Button variant="secondary" size="sm" onClick={() => setSelecting(true)}>
                选择
              </Button>
            ) : null}
            <Link to="/courses/new" className={buttonClasses({ size: 'sm' })}>
              <Plus aria-hidden="true" className="h-4 w-4" />
              新建课程
            </Link>
          </div>
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
            {selecting ? (
              <SelectionToolbar
                total={courses.length}
                selected={selectedIds.size}
                onToggleAll={(checked) =>
                  setSelectedIds(checked ? new Set(courses.map((course) => course.id)) : new Set())
                }
                onDelete={() => setDeleteOpen(true)}
                onCancel={exitSelecting}
              />
            ) : null}
            <ul className="divide-y divide-border rounded-md border border-border bg-surface">
              {courses.map((course) => (
                <li key={course.id}>
                  {selecting ? (
                    <label className="group flex cursor-pointer items-center gap-4 px-4 py-3 hover:bg-surface-sunken">
                      <input
                        type="checkbox"
                        aria-label={`选择 ${course.title}`}
                        checked={selectedIds.has(course.id)}
                        onChange={() => toggleSelected(course.id)}
                        className="shrink-0 accent-accent"
                      />
                      <CourseRowContent course={course} />
                    </label>
                  ) : (
                    <div className="flex items-center pr-3">
                      <Link
                        to="/courses/$courseId"
                        params={{ courseId: course.id }}
                        className="group flex min-w-0 flex-1 items-center gap-4 px-4 py-3 hover:bg-surface-sunken"
                      >
                        <CourseRowContent course={course} />
                        <ChevronRight
                          aria-hidden="true"
                          className="h-4 w-4 shrink-0 text-text-subtle group-hover:text-accent"
                        />
                      </Link>
                      <DeleteContentButton kind="course" id={course.id} name={course.title} />
                    </div>
                  )}
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
            <ConfirmDialog
              open={deleteOpen}
              onOpenChange={setDeleteOpen}
              title="删除课程"
              description={`将删除所选 ${selectedIds.size} 门课程及其全部课时、素材、文档、知识点、测验、作答、总结和薄弱点，此操作不可恢复。`}
              confirmLabel="删除"
              danger
              busy={batchDelete.isPending}
              errorMessage={deleteError ?? undefined}
              onConfirm={() => void confirmDelete()}
            />
          </>
        )
      ) : null}
    </div>
  )
}

/** 课程行内容：普通模式包在 Link 里，选择模式包在 label 里，结构保持一致。 */
function CourseRowContent({ course }: { course: Course }) {
  return (
    <>
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
        <span className="mt-0.5 block">更新于 {formatDateTime(course.updated_at)}</span>
      </span>
    </>
  )
}
