import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { useState } from 'react'

import { createAppQueryClient } from '@/api/query-client'
import { ErrorBoundary } from '@/components/error-boundary'
import { JobStreamProvider } from '@/features/jobs/job-stream-provider'
import { router } from '@/router'

export function App() {
  // QueryClient 全局唯一实例；业务实体只存于 Query Cache，不进全局 Store。
  const [queryClient] = useState(createAppQueryClient)

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <JobStreamProvider>
          <RouterProvider router={router} />
        </JobStreamProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  )
}
