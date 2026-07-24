import {
  Content,
  Description,
  Overlay,
  Portal,
  Root,
  Title,
  Trigger,
  Close,
} from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/cn'

/**
 * Radix Dialog 的项目样式封装：焦点圈禁、Esc 关闭、关闭后焦点回到触发器
 * 都由 Radix 保证；这里只负责视觉。动效仅 motion-safe 下启用。
 */
export const Dialog = Root
export const DialogTrigger = Trigger
export const DialogClose = Close

export interface DialogContentProps {
  title: string
  description?: string
  children: ReactNode
  className?: string
}

export function DialogContent({ title, description, children, className }: DialogContentProps) {
  return (
    <Portal>
      <Overlay className="fixed inset-0 z-40 bg-text/40" />
      <Content
        className={cn(
          'fixed left-1/2 top-1/2 z-50 max-h-[85dvh] w-[calc(100vw-2rem)] max-w-lg',
          '-translate-x-1/2 -translate-y-1/2 overflow-y-auto',
          'rounded-lg border border-border bg-surface p-4 shadow-dialog',
          className,
        )}
      >
        <div className="flex items-start justify-between gap-4">
          <Title className="text-base font-semibold text-text">{title}</Title>
          <Close
            aria-label="关闭对话框"
            className="rounded-md p-1 text-text-subtle hover:bg-surface-sunken hover:text-text"
          >
            <X aria-hidden="true" className="h-4 w-4" />
          </Close>
        </div>
        {description ? (
          <Description className="mt-1 text-sm text-text-muted">{description}</Description>
        ) : null}
        <div className="mt-4">{children}</div>
      </Content>
    </Portal>
  )
}
