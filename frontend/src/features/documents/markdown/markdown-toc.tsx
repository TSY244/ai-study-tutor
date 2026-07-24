import { useState } from 'react'

import type { TocHeading } from './heading-utils'

/**
 * 文档目录。桌面端是右侧粘性栏，窄屏是内容上方的折叠面板（原生
 * details/summary，键盘与屏幕阅读器默认可用）。同一时刻只有一种形态
 * 可见（另一种 display:none，不进 a11y 树），不产生重复导航。
 */
export function MarkdownToc({
  headings,
  variant,
}: {
  headings: TocHeading[]
  variant: 'mobile' | 'desktop'
}) {
  const [open, setOpen] = useState(false)

  const list = (onNavigate?: () => void) => (
    <ul className="space-y-1 text-sm">
      {headings.map((heading) => (
        <li
          key={heading.id}
          style={{ paddingLeft: `${Math.min(Math.max(heading.depth - 1, 0), 2) * 0.75}rem` }}
        >
          <a
            href={`#${heading.id}`}
            onClick={onNavigate}
            className="block rounded-sm py-0.5 text-text-muted hover:text-text"
          >
            {heading.text}
          </a>
        </li>
      ))}
    </ul>
  )

  if (variant === 'mobile') {
    return (
      <details
        open={open}
        onToggle={(event) => setOpen(event.currentTarget.open)}
        className="rounded-md border border-border bg-surface xl:hidden"
      >
        <summary className="cursor-pointer rounded-md px-3 py-2 text-sm font-medium text-text">
          目录
        </summary>
        <nav aria-label="文档目录" className="border-t border-border px-3 py-2">
          {list(() => setOpen(false))}
        </nav>
      </details>
    )
  }

  return (
    <aside className="hidden w-56 shrink-0 xl:block">
      <nav aria-label="文档目录" className="sticky top-4 max-h-[calc(100dvh-6rem)] overflow-y-auto">
        <p className="mb-2 text-xs font-medium uppercase tracking-wide text-text-subtle">目录</p>
        {list()}
      </nav>
    </aside>
  )
}
