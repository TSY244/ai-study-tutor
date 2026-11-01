import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from '@/app'
import { env } from '@/env'
// 所有入口（含直接打开专注作答页）都需要接收跨标签页主题变化。
import '@/lib/theme'
import '@/styles/globals.css'

async function bootstrap(): Promise<void> {
  // 开发接线：后端未就绪时用 MSW 接管 /api（VITE_ENABLE_MSW=true，仅 DEV）。
  // import.meta.env 必须直接出现在此判断中，生产构建才能静态消除 MSW 分支，
  // 保证 mock 代码（含 faker）不进入生产 dist。
  if (import.meta.env.DEV && import.meta.env.VITE_ENABLE_MSW === 'true') {
    const { worker } = await import('@/mocks/browser')
    await worker.start({ onUnhandledRequest: 'bypass', quiet: true })
  }

  document.title = env.appTitle

  const container = document.getElementById('root')
  if (!container) throw new Error('找不到 #root 挂载点')
  createRoot(container).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}

void bootstrap()
