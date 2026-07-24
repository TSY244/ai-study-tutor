import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/cn'

export interface EmptyStateProps {
  icon?: LucideIcon
  title: string
  /** 说明为什么为空。 */
  description?: string
  /** 只放一个最相关的下一步操作。 */
  action?: ReactNode
  className?: string
}

/** 空状态框架：说明原因 + 唯一下一步。 */
export function EmptyState({ icon: Icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center rounded-md border border-dashed border-border-strong bg-surface px-6 py-10 text-center',
        className,
      )}
    >
      {Icon ? (
        <span className="mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-surface-sunken">
          <Icon aria-hidden="true" className="h-5 w-5 text-text-subtle" />
        </span>
      ) : null}
      <p className="font-medium text-text">{title}</p>
      {description ? <p className="mt-1 max-w-md text-sm text-text-muted">{description}</p> : null}
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  )
}
