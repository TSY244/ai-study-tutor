import { createFileRoute } from '@tanstack/react-router'

import { DocumentView } from '@/features/documents/document-view'

export const Route = createFileRoute('/_shell/courses/$courseId/lessons/$lessonId/raw')({
  staticData: { crumb: '原文' },
  component: LessonRawPage,
})

function LessonRawPage() {
  const { lessonId } = Route.useParams()
  return <DocumentView lessonId={lessonId} kind="raw_text" />
}
