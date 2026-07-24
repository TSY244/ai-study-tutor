import { createRootRoute, Outlet } from '@tanstack/react-router'

import { ErrorFallback } from '@/components/error-boundary'

/**
 * 根路由：只负责分发。常规页面由 _shell 布局提供 App Shell；
 * /attempts/:id 使用专注模式布局（无全局导航）。
 */
export const Route = createRootRoute({
  component: () => <Outlet />,
  errorComponent: ({ error, reset }) => <ErrorFallback error={error} onReset={reset} />,
  notFoundComponent: NotFound,
})

function NotFound() {
  return (
    <div className="mx-auto max-w-lg px-4 py-16">
      <h1 className="text-lg font-semibold text-text">页面不存在</h1>
      <p className="mt-2 text-text-muted">你访问的地址没有对应的内容。</p>
      <p className="mt-6">
        <a href="/dashboard" className="font-medium text-accent underline-offset-2 hover:underline">
          返回学习总览
        </a>
      </p>
    </div>
  )
}
