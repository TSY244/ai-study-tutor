import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createMemoryHistory, createRouter, RouterProvider } from '@tanstack/react-router'
import { render } from '@testing-library/react'
import type { ReactElement, ReactNode } from 'react'

import { routeTree } from '@/routeTree.gen'

/** 测试用 QueryClient：不重试、不缓存，结果即时可见。 */
export function createTestQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Number.POSITIVE_INFINITY, staleTime: 0 },
      mutations: { retry: false },
    },
  })
}

export function renderWithClient(ui: ReactElement, client = createTestQueryClient()) {
  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }
  return { client, ...render(ui, { wrapper: Wrapper }) }
}

/**
 * 用真实路由树 + 内存 history 渲染整个应用：Shell、面包屑、路由组件
 * 都在产品同样的组装下被测，数据走 MSW curated fixtures。
 */
export function renderWithRouter(initialPath: string) {
  const client = createTestQueryClient()
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
  })
  const result = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { client, router, ...result }
}
