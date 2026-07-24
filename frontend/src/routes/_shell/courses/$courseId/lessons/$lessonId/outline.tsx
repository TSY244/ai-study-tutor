import { createFileRoute } from '@tanstack/react-router'

import { DocumentView } from '@/features/documents/document-view'

export const Route = createFileRoute('/_shell/courses/$courseId/lessons/$lessonId/outline')({
  staticData: { crumb: '大纲' },
  component: LessonOutlinePage,
})

function LessonOutlinePage() {
  const { lessonId } = Route.useParams()
  return <DocumentView lessonId={lessonId} kind="teacher_outline" />
}
