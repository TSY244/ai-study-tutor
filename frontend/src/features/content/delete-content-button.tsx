import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2 } from 'lucide-react'
import { useState } from 'react'

import { deleteAttempt } from '@/api/generated/endpoints/attempts/attempts'
import { deleteCourse, deleteKnowledgePoint } from '@/api/generated/endpoints/courses/courses'
import { deleteDocument } from '@/api/generated/endpoints/documents/documents'
import { deleteLesson } from '@/api/generated/endpoints/lessons/lessons'
import { deleteMaterial } from '@/api/generated/endpoints/materials/materials'
import { deleteQuiz } from '@/api/generated/endpoints/quizzes/quizzes'
import { deleteSummaryEvaluation } from '@/api/generated/endpoints/summaries/summaries'
import { deleteWeakness } from '@/api/generated/endpoints/weaknesses/weaknesses'
import { invalidateCourseRemoved, invalidateLessonRemoved } from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { clearAttemptDraft } from '@/features/attempts/attempt-draft'
import { removeRecentLearningEntries } from '@/features/learning/recent-learning'

const resources = {
  course: {
    label: '课程',
    remove: deleteCourse,
    description:
      '课程及其全部课时、素材、所有文档版本、知识点、测验、作答、总结和薄弱点将一并删除，无法撤销。',
  },
  lesson: {
    label: '课时',
    remove: deleteLesson,
    description:
      '课时及其素材、全部文档版本、知识点、测验、作答、总结和课时薄弱点将一并删除，学习进度会重新计算，无法撤销。',
  },
  quiz: {
    label: '测验',
    remove: deleteQuiz,
    description:
      '测验及其作答记录、答案、评分和相关进度快照将一并删除，学习进度会重新计算，无法撤销。',
  },
  knowledge: {
    label: '知识点',
    remove: deleteKnowledgePoint,
    description: '此知识点将从课程中移除，已有测验中的知识点快照会保留。',
  },
  document: {
    label: '文档',
    remove: deleteDocument,
    description:
      '仅删除当前查看的这个版本，其他版本保留；删除当前版本后不会自动恢复旧版。删除原文还会使相关知识点失效。',
  },
  material: {
    label: '素材',
    remove: deleteMaterial,
    description: '删除此来源素材，已生成的文档和测验会保留，无法撤销。',
  },
  summary: {
    label: '总结',
    remove: deleteSummaryEvaluation,
    description: '删除这次提交的总结及其评估结果，无法撤销；输入框中的草稿会保留。',
  },
  attempt: {
    label: '作答记录',
    remove: deleteAttempt,
    description: '删除这次作答、答案和评分，相关学习进度会重新计算，无法撤销。',
  },
  weakness: {
    label: '薄弱点',
    remove: deleteWeakness,
    description: '删除此薄弱点及其复习记录，相关学习进度会重新计算，无法撤销。',
  },
} as const

interface Props {
  kind: keyof typeof resources
  id: string
  name: string
  courseId?: string
  lessonId?: string
  onDeleted?: () => void | Promise<void>
}

/** 列表与内容页共用的显式删除入口；确认成功后才刷新对应资源。 */
export function DeleteContentButton({ kind, id, name, courseId, lessonId, onDeleted }: Props) {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const resource = resources[kind]
  const mutation = useMutation({
    mutationFn: () => resource.remove(id),
    onSuccess: async () => {
      if (kind === 'course') {
        removeRecentLearningEntries({ courseId: id })
        await invalidateCourseRemoved(queryClient, id)
      } else if (kind === 'lesson' && courseId) {
        removeRecentLearningEntries({ lessonId: id })
        await invalidateLessonRemoved(queryClient, id, courseId)
      } else {
        const prefixes: readonly unknown[][] =
          {
            lesson: [],
            quiz: [
              ['quizzes', lessonId],
              ['quiz', id],
              ['lesson-attempts', lessonId],
              ['attempt'],
              ['weaknesses'],
              ['progress'],
              ['lesson', lessonId],
            ],
            knowledge: [['knowledge-points', courseId]],
            document: [
              ['documents', lessonId],
              ['document', lessonId],
              ['document-versions', lessonId],
              ['lesson', lessonId],
              ['knowledge-points'],
            ],
            material: [['materials', lessonId]],
            summary: [['summary-evaluations', lessonId]],
            attempt: [
              ['lesson-attempts', lessonId],
              ['weaknesses'],
              ['progress'],
              ['course', courseId],
              ['lesson', lessonId],
            ],
            weakness: [['weaknesses'], ['progress']],
          }[kind] ?? []
        if (kind === 'attempt') {
          clearAttemptDraft(id)
          queryClient.removeQueries({ queryKey: ['attempt', id], exact: true })
        }
        if (kind === 'document') {
          queryClient.removeQueries({ queryKey: ['document-version', id], exact: true })
        }
        await Promise.all(prefixes.map((queryKey) => queryClient.invalidateQueries({ queryKey })))
      }
      await onDeleted?.()
      setOpen(false)
    },
  })
  const message = mutation.isError
    ? isApiProblemError(mutation.error) && mutation.error.code === 'resource_busy'
      ? '相关任务正在处理，请等待任务完成或取消后再删除。'
      : presentProblem(mutation.error).message
    : undefined
  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        className="shrink-0 text-danger hover:text-danger"
        aria-label={`删除${resource.label}「${name}」`}
        onClick={() => {
          mutation.reset()
          setOpen(true)
        }}
      >
        <Trash2 aria-hidden="true" className="h-4 w-4" />
        删除
      </Button>
      <ConfirmDialog
        open={open}
        onOpenChange={(next) => {
          if (!mutation.isPending) setOpen(next)
        }}
        title={`删除${resource.label}「${name}」？`}
        description={resource.description}
        confirmLabel="删除"
        danger
        busy={mutation.isPending}
        errorMessage={message}
        onConfirm={() => mutation.mutate()}
      />
    </>
  )
}
