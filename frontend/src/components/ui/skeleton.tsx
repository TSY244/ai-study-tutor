import { cn } from '@/lib/cn'

/**
 * 加载占位：形状贴近真实内容，不用全屏旋转图标覆盖已有内容。
 * 容器需自行设置 aria-busy 与 aria-label。
 */
export function Skeleton({ className }: { className?: string }) {
  return (
    <div aria-hidden="true" className={cn('animate-pulse rounded bg-surface-sunken', className)} />
  )
}

/** 多行文本占位。 */
export function SkeletonText({ lines = 3, className }: { lines?: number; className?: string }) {
  const widths = ['w-full', 'w-full', 'w-2/3', 'w-4/5', 'w-1/2']
  return (
    <div aria-hidden="true" className={cn('space-y-2', className)}>
      {Array.from({ length: lines }, (_, i) => (
        <div
          key={i}
          className={cn('h-4 animate-pulse rounded bg-surface-sunken', widths[i % widths.length])}
        />
      ))}
    </div>
  )
}

/** 列表行占位（标题 + 次行）。 */
export function SkeletonRows({ rows = 3, className }: { rows?: number; className?: string }) {
  return (
    <div
      aria-hidden="true"
      className={cn('divide-y divide-border rounded-md border border-border bg-surface', className)}
    >
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="space-y-2 px-4 py-3">
          <div className="h-4 w-1/3 animate-pulse rounded bg-surface-sunken" />
          <div className="h-3 w-2/3 animate-pulse rounded bg-surface-sunken" />
        </div>
      ))}
    </div>
  )
}
