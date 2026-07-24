import { createFileRoute, redirect } from '@tanstack/react-router'

// 课时默认子视图：笔记。
export const Route = createFileRoute('/_shell/courses/$courseId/lessons/$lessonId/')({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: '/courses/$courseId/lessons/$lessonId/notes',
      params,
      replace: true,
    })
  },
})
