import { Hammer } from 'lucide-react'

import { EmptyState } from '@/components/ui/empty-state'

/**
 * 阶段占位面板：路由/骨架先行的页面统一用它，说明该模块的
 * 计划内容。随各阶段交付逐个替换为真实实现。
 */
export function FeaturePending({ title, detail }: { title: string; detail: string }) {
  return (
    <EmptyState
      icon={Hammer}
      title={`${title}将在后续阶段提供`}
      description={detail}
      className="bg-canvas"
    />
  )
}
