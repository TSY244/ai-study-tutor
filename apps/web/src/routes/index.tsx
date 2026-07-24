import { createFileRoute, redirect } from '@tanstack/react-router'

// 首屏即产品：根路径直接进入学习总览，不做营销首页。
export const Route = createFileRoute('/')({
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' })
  },
})
