import { createFileRoute } from '@tanstack/react-router'

import { SummaryPanel } from '@/features/summaries/summary-panel'

export const Route = createFileRoute('/_shell/courses/$courseId/lessons/$lessonId/summary')({
  staticData: { crumb: '总结' },
  component: LessonSummaryPage,
})

function LessonSummaryPage() {
  const { lessonId } = Route.useParams()
  return <SummaryPanel lessonId={lessonId} />
}
