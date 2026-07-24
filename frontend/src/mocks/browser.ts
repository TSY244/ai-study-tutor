import { setupWorker } from 'msw/browser'

import { __testPeekJobs, handlers, jobStreamMockController } from '@/mocks/handlers'

export const worker = setupWorker(...handlers)

// E2E 测试钩子（仅 DEV mock 接线存在，生产构建被静态消除）：
// 推送/断开任务事件流，窥视任务存储以定位服务端创建的 Job id。
window.__jobStreamMock = jobStreamMockController
window.__peekJobs = __testPeekJobs
