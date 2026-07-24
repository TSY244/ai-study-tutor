import { createFileRoute } from '@tanstack/react-router'

import { AppShell } from '@/components/app-shell/app-shell'

// 路径无关布局：URL 不含 _shell，其所有子路由共享 App Shell。
export const Route = createFileRoute('/_shell')({
  component: AppShell,
})
