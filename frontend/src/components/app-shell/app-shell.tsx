import { Link, Outlet } from '@tanstack/react-router'

import { Breadcrumbs } from '@/components/app-shell/breadcrumbs'
import { CommandPalette } from '@/components/app-shell/command-palette'
import { JobsCenter } from '@/components/app-shell/jobs-center'
import { env } from '@/env'
import { cn } from '@/lib/cn'

const NAV_ITEMS = [
  { to: '/dashboard', label: '学习总览', exact: true },
  { to: '/courses', label: '课程', exact: false },
  { to: '/weaknesses', label: '弱点', exact: false },
  { to: '/progress', label: '进度', exact: false },
  { to: '/settings', label: '设置', exact: true },
] as const

/**
 * 全局产品壳：skip link -> 顶栏（标题 / 主导航 / 命令面板 / 任务中心）->
 * 面包屑 -> 主内容。任务中心入口常驻顶栏右侧，全页面可见。
 * 考试作答路由（/attempts/:id）不走此壳，使用专注模式布局。
 */
export function AppShell() {
  return (
    <div className="flex min-h-dvh flex-col">
      <a href="#main-content" className="skip-link">
        跳到主要内容
      </a>
      <header className="border-b border-border bg-surface">
        <div className="mx-auto flex h-12 max-w-6xl items-center gap-3 px-4 xl:max-w-7xl 2xl:max-w-[92rem]">
          <Link
            to="/dashboard"
            className="shrink-0 rounded-sm font-semibold text-text"
            aria-label={`${env.appTitle} 学习总览`}
          >
            {env.appTitle}
          </Link>
          <nav aria-label="主导航" className="min-w-0 flex-1 overflow-x-auto">
            <ul className="flex items-center gap-1 text-sm">
              {NAV_ITEMS.map((item) => (
                <li key={item.to} className="shrink-0">
                  <Link
                    to={item.to}
                    activeOptions={{ exact: item.exact }}
                    activeProps={{
                      'aria-current': 'page',
                      className: 'bg-accent-subtle font-medium text-accent-strong',
                    }}
                    inactiveProps={{
                      className: 'text-text-muted hover:bg-surface-sunken hover:text-text',
                    }}
                    className="block rounded-md px-2 py-1.5 transition-colors duration-120"
                  >
                    {item.label}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
          <div className="flex shrink-0 items-center gap-1.5">
            <CommandPalette />
            <JobsCenter />
          </div>
        </div>
      </header>
      <Breadcrumbs />
      <main
        id="main-content"
        tabIndex={-1}
        className={cn(
          'mx-auto w-full max-w-6xl flex-1 px-4 py-5 outline-none xl:max-w-7xl 2xl:max-w-[92rem]',
        )}
      >
        <Outlet />
      </main>
    </div>
  )
}
