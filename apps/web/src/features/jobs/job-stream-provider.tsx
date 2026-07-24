import { useQueryClient } from '@tanstack/react-query'
import { useEffect, type ReactNode } from 'react'

import { JobStreamClient } from '@/api/job-stream'

/**
 * 全局任务事件流：App 级单例，随 QueryClient 生命周期启停。
 * 专注作答模式也在其覆盖下（批阅结果依赖 SSE 推送）。
 * 连接语义（游标/退避/resync/心跳）全部在 JobStreamClient 内。
 */
export function JobStreamProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()

  useEffect(() => {
    const client = new JobStreamClient({ queryClient })
    client.start()
    return () => client.stop()
  }, [queryClient])

  return children
}
