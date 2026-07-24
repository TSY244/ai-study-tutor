import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import type { Course } from '@/api/generated/model'
import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { EditCourseDialog } from '@/features/courses/edit-course-dialog'
import { useDeleteCourse, useUpdateCourse } from '@/features/courses/use-course-mutations'

/**
 * 课程详情页操作组：编辑（对话框表单）、归档/恢复、删除（确认框）。
 * 归档/删除失败时错误留在确认框内，不打断上下文。
 */
export function CourseActions({ course }: { course: Course }) {
  const navigate = useNavigate()
  const [editOpen, setEditOpen] = useState(false)
  const [archiveOpen, setArchiveOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [archiveError, setArchiveError] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const updateCourse = useUpdateCourse(course.id)
  const deleteCourse = useDeleteCourse(course.id)

  const confirmArchive = async () => {
    setArchiveError(null)
    try {
      await updateCourse.mutateAsync({ archived: !course.archived, version: course.version })
      setArchiveOpen(false)
    } catch (error) {
      const presentation = presentProblem(error)
      setArchiveError(`${presentation.title}：${presentation.message}`)
    }
  }

  const confirmDelete = async () => {
    setDeleteError(null)
    try {
      await deleteCourse.mutateAsync()
      setDeleteOpen(false)
      await navigate({ to: '/courses' })
    } catch (error) {
      const presentation = presentProblem(error)
      setDeleteError(`${presentation.title}：${presentation.message}`)
    }
  }

  return (
    <div className="flex items-center gap-2">
      <Button variant="secondary" size="sm" onClick={() => setEditOpen(true)}>
        编辑
      </Button>
      <Button variant="secondary" size="sm" onClick={() => setArchiveOpen(true)}>
        {course.archived ? '恢复' : '归档'}
      </Button>
      <Button variant="danger" size="sm" onClick={() => setDeleteOpen(true)}>
        删除
      </Button>

      <EditCourseDialog course={course} open={editOpen} onOpenChange={setEditOpen} />
      <ConfirmDialog
        open={archiveOpen}
        onOpenChange={setArchiveOpen}
        title={course.archived ? '恢复课程' : '归档课程'}
        description={
          course.archived
            ? `恢复后「${course.title}」重新出现在活跃课程中。`
            : `归档后「${course.title}」标记为不活跃，课时与数据保留。`
        }
        confirmLabel={course.archived ? '恢复' : '归档'}
        busy={updateCourse.isPending}
        errorMessage={archiveError ?? undefined}
        onConfirm={() => void confirmArchive()}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="删除课程"
        description={`将删除「${course.title}」及其全部课时与素材，此操作不可恢复。`}
        confirmLabel="删除"
        danger
        busy={deleteCourse.isPending}
        errorMessage={deleteError ?? undefined}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  )
}
