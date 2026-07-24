import { cn } from '@/lib/cn'

export type BadgeVariant = 'neutral' | 'ok' | 'warn' | 'danger' | 'info' | 'accent'

const VARIANT_CLASSES: Record<BadgeVariant, string> = {
  neutral: 'bg-surface-sunken text-text-muted',
  ok: 'bg-ok-bg text-ok',
  warn: 'bg-warn-bg text-warn',
  danger: 'bg-danger-bg text-danger',
  info: 'bg-info-bg text-info',
  accent: 'bg-accent-bg text-accent-strong',
}

export interface BadgeProps {
  variant?: BadgeVariant
  /** 文本标签必填：状态不只靠颜色区分。 */
  children: string
  className?: string
}

/** 状态/分类小标签。纯 span，可嵌入任意文本行。 */
export function Badge({ variant = 'neutral', children, className }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center gap-1 rounded-sm px-1.5 py-0.5 text-xs font-medium',
        VARIANT_CLASSES[variant],
        className,
      )}
    >
      {children}
    </span>
  )
}
