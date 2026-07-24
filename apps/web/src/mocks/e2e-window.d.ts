import type { Job } from '@/api/generated/model'
import type { JobStreamMockController } from '@/mocks/job-stream-mock'

/**
 * E2E 测试钩子（仅 DEV + VITE_ENABLE_MSW 接线时存在于 window）：
 * - __jobStreamMock：任务事件流控制器（push/closeAll/connections/lastCursor）。
 * - __peekJobs：任务存储只读快照，用于定位服务端创建的 Job id。
 */
declare global {
  interface Window {
    __jobStreamMock?: JobStreamMockController
    __peekJobs?: () => Job[]
  }
}

export {}
