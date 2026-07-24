import { createFileRoute, Outlet } from '@tanstack/react-router'

// 任务中心层级锚点。
export const Route = createFileRoute('/_shell/jobs')({
  staticData: { crumb: '任务中心' },
  component: () => <Outlet />,
})
