import { createFileRoute } from '@tanstack/react-router'

import { FeaturePending } from '@/components/feature-pending'

export const Route = createFileRoute('/_shell/courses/$courseId/lessons/$lessonId/summary')({
  staticData: { crumb: '总结' },
  component: LessonSummaryPage,
})

function LessonSummaryPage() {
  return <FeaturePending title="学习总结" detail="手写总结与 AI 评估反馈将在 F4/F6 实现。" />
}
