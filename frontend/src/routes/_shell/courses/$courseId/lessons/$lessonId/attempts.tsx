import { createFileRoute, Link } from '@tanstack/react-router'

import { EmptyState } from '@/components/ui/empty-state'

export const Route = createFileRoute('/_shell/courses/$courseId/lessons/$lessonId/attempts')({
  staticData: { crumb: '作答' },
  component: LessonAttemptsPage,
})

/**
 * 作答记录页：契约目前没有「按课时列出 Attempt」的端点
 * （api/openapi.yaml 仅有 create/get/save/submit/grading-jobs）。
 * 历史作答列表在契约补齐前阻塞（F5 报告已标记）；进行中的作答
 * 从测验页进入，批阅完成的任务从任务中心进入。
 */
function LessonAttemptsPage() {
  const { courseId, lessonId } = Route.useParams()
  return (
    <EmptyState
      title="作答记录列表待契约补齐"
      description="接口尚不支持按课时查询历史作答。可从测验页开始新的作答；批阅中的作答会在任务中心出现。"
      action={
        <Link
          to="/courses/$courseId/lessons/$lessonId/quizzes"
          params={{ courseId, lessonId }}
          className="rounded-md bg-accent px-3 py-1.5 text-sm font-medium text-accent-contrast hover:bg-accent-strong"
        >
          前往测验
        </Link>
      }
    />
  )
}
