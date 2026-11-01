import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'

import { resetAttemptDrafts } from '@/features/attempts/attempt-draft'
import { resetRecentLearning } from '@/features/learning/recent-learning'
import { resetLastEvaluationIds } from '@/features/summaries/last-evaluation'
import { resetIdempotencyIntents } from '@/lib/idempotency'
import { resetTheme } from '@/lib/theme'
import { resetMockState } from '@/mocks/handlers'
import { server } from '@/mocks/server'

// jsdom 未实现 scrollTo；TanStack Router 滚动恢复会调用它，打桩避免噪音。
window.scrollTo = () => {}

// findBy/waitFor 默认 1s 超时，在 CPU 受限的 Docker 构建环境容易抖动，放宽到 5s。
configure({ asyncUtilTimeout: 5000 })

// MSW 接管所有 fetch；未接线的请求直接报错，迫使测试显式声明依赖的 API。
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => {
  cleanup()
  server.resetHandlers()
  resetMockState()
  resetIdempotencyIntents()
  resetTheme()
  resetAttemptDrafts()
  resetRecentLearning()
  resetLastEvaluationIds()
})
afterAll(() => server.close())
