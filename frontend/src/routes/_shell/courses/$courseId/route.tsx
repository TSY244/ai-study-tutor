import { createFileRoute, Outlet } from '@tanstack/react-router'

// 单个课程的层级锚点：crumb '$course' 由面包屑替换为课程标题。
export const Route = createFileRoute('/_shell/courses/$courseId')({
  staticData: { crumb: '$course' },
  component: () => <Outlet />,
})
