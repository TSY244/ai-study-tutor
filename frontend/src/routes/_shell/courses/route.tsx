import { createFileRoute, Outlet } from '@tanstack/react-router'

// 课程资源布局：仅提供面包屑锚点与路由层级，页面各自渲染。
export const Route = createFileRoute('/_shell/courses')({
  staticData: { crumb: '课程' },
  component: () => <Outlet />,
})
