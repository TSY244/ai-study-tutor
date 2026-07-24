import { createFileRoute } from '@tanstack/react-router'

import { DocumentView } from '@/features/documents/document-view'

export const Route = createFileRoute('/_shell/courses/$courseId/lessons/$lessonId/notes')({
  staticData: { crumb: '笔记' },
  component: LessonNotesPage,
})

function LessonNotesPage() {
  const { lessonId } = Route.useParams()
  return <DocumentView lessonId={lessonId} kind="student_notes" />
}
