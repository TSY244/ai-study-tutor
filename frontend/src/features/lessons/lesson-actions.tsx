import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import type { Lesson } from '@/api/generated/model'
import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { EditLessonDialog } from '@/features/lessons/edit-lesson-dialog'
import { useDeleteLesson } from '@/features/lessons/use-lesson-mutations'

/** 课时操作组：编辑（对话框）、删除（确认框，删除后回到课程页）。 */
export function LessonActions({ lesson }: { lesson: Lesson }) {
  const navigate = useNavigate()
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const deleteLesson = useDeleteLesson(lesson.id, lesson.course_id)

  const confirmDelete = async () => {
    setDeleteError(null)
    try {
      await deleteLesson.mutateAsync()
      setDeleteOpen(false)
      await navigate({ to: '/courses/$courseId', params: { courseId: lesson.course_id } })
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
      <Button variant="danger" size="sm" onClick={() => setDeleteOpen(true)}>
        删除
      </Button>

      <EditLessonDialog lesson={lesson} open={editOpen} onOpenChange={setEditOpen} />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="删除课时"
        description={`将删除「${lesson.title}」及其素材与生成内容，此操作不可恢复。`}
        confirmLabel="删除"
        danger
        busy={deleteLesson.isPending}
        errorMessage={deleteError ?? undefined}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  )
}
