import { DeleteContentButton } from '@/features/content/delete-content-button'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { BookOpen, ChevronRight, Plus } from 'lucide-react'
import { useState } from 'react'

import { getCourse } from '@/api/generated/endpoints/courses/courses'
import { listLessons } from '@/api/generated/endpoints/lessons/lessons'
import type { Lesson } from '@/api/generated/model'
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
import { Skeleton, SkeletonRows } from '@/components/ui/skeleton'
import { KnowledgePoints } from '@/features/knowledge/knowledge-points'
import { CourseActions } from '@/features/courses/course-actions'
import { useBatchDeleteLessons } from '@/features/lessons/use-lesson-mutations'
import { formatDateTime } from '@/lib/format'
import { LESSON_STATUS_META } from '@/lib/status'

const PAGE_SIZE = 50

/**
 * 课程详情：课程信息 + 操作（编辑/归档/删除）+ 课时列表（游标分页）。
 * 新建课时进入独立表单页；课时导入在课时页完成。
 */
export const Route = createFileRoute('/_shell/courses/$courseId/')({
  // 课程参数变化时重建页面，避免沿用其他课程的选择与删除对话框。
  remountDeps: ({ params }) => params.courseId,
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

  // 多选删除：与课程列表页同一模式——选择模式下整行变为 label + checkbox，
  // 删除走原子批量接口，409 resource_busy 等错误留在确认框内呈现。
  // Hooks 必须在上方 isPending/isError 提前返回之前声明。
  const [selecting, setSelecting] = useState(false)
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<string>>(new Set())
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const batchDelete = useBatchDeleteLessons(courseId)

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

  const toggleSelected = (lessonId: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(lessonId)) {
        next.delete(lessonId)
      } else {
        next.add(lessonId)
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
        title={course.title}
        description={course.description || undefined}
        actions={
          <div className="flex items-center gap-2">
            {course.archived ? <Badge>已归档</Badge> : null}
            <CourseActions course={course} />
          </div>
        }
      />

      <KnowledgePoints courseId={courseId} />
      <section aria-labelledby="lessons-heading">
        <div className="flex items-center justify-between gap-3">
          <h2 id="lessons-heading" className="text-base font-medium text-text">
            课时
          </h2>
          <div className="flex items-center gap-2">
            {lessonsQuery.isSuccess && lessons.length > 0 && !selecting ? (
              <Button variant="secondary" size="sm" onClick={() => setSelecting(true)}>
                选择
              </Button>
            ) : null}
            <Link
              to="/courses/$courseId/lessons/new"
              params={{ courseId }}
              className={buttonClasses({ variant: 'secondary', size: 'sm' })}
            >
              <Plus aria-hidden="true" className="h-4 w-4" />
              新建课时
            </Link>
          </div>
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
                {selecting ? (
                  <div className="mb-3">
                    <SelectionToolbar
                      total={lessons.length}
                      selected={selectedIds.size}
                      onToggleAll={(checked) =>
                        setSelectedIds(
                          checked ? new Set(lessons.map((lesson) => lesson.id)) : new Set(),
                        )
                      }
                      onDelete={() => setDeleteOpen(true)}
                      onCancel={exitSelecting}
                    />
                  </div>
                ) : null}
                <ul className="divide-y divide-border rounded-md border border-border bg-surface">
                  {lessons.map((lesson) => (
                    <li key={lesson.id}>
                      {selecting ? (
                        <label className="group flex cursor-pointer items-center gap-4 px-4 py-3 hover:bg-surface-sunken">
                          <input
                            type="checkbox"
                            aria-label={`选择 ${lesson.title}`}
                            checked={selectedIds.has(lesson.id)}
                            onChange={() => toggleSelected(lesson.id)}
                            className="shrink-0 accent-accent"
                          />
                          <LessonRowContent lesson={lesson} />
                        </label>
                      ) : (
                        <div className="flex items-center pr-3">
                          <Link
                            to="/courses/$courseId/lessons/$lessonId"
                            params={{ courseId, lessonId: lesson.id }}
                            className="group flex min-w-0 flex-1 items-center gap-4 px-4 py-3 hover:bg-surface-sunken"
                          >
                            <LessonRowContent lesson={lesson} />
                            <ChevronRight
                              aria-hidden="true"
                              className="h-4 w-4 shrink-0 text-text-subtle group-hover:text-accent"
                            />
                          </Link>
                          <DeleteContentButton
                            kind="lesson"
                            id={lesson.id}
                            name={lesson.title}
                            courseId={courseId}
                          />
                        </div>
                      )}
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
                <ConfirmDialog
                  open={deleteOpen}
                  onOpenChange={setDeleteOpen}
                  title="删除课时"
                  description={`将删除所选 ${selectedIds.size} 个课时及其素材、全部文档、知识点、测验、作答、总结和课时薄弱点，此操作不可恢复。`}
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
      </section>

      <p className="text-xs text-text-subtle">
        更新于 {formatDateTime(course.updated_at)} · 版本 {course.version}
      </p>
    </div>
  )
}

/** 课时行内容：普通模式包在 Link 里，选择模式包在 label 里，结构保持一致。 */
function LessonRowContent({ lesson }: { lesson: Lesson }) {
  return (
    <>
      <span className="w-14 shrink-0 text-sm text-text-subtle">第 {lesson.ordinal} 课</span>
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
        {typeof lesson.mastery === 'number' ? `掌握度 ${Math.round(lesson.mastery * 100)}%` : ''}
      </span>
    </>
  )
}
