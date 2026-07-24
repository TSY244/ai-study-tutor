import { Content, Overlay, Portal, Root, Title, Trigger } from '@radix-ui/react-dialog'
import { useNavigate } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useEffect, useId, useMemo, useState, type KeyboardEvent } from 'react'

import { buttonClasses } from '@/components/ui/button-styles'
import { cn } from '@/lib/cn'

const COMMANDS = [
  { id: 'dashboard', label: '学习总览', keywords: 'dashboard overview 总览', to: '/dashboard' },
  { id: 'courses', label: '课程列表', keywords: 'courses 课程', to: '/courses' },
  { id: 'course-new', label: '新建课程', keywords: 'create course 创建 新建', to: '/courses/new' },
  { id: 'jobs', label: '任务中心', keywords: 'jobs tasks 任务', to: '/jobs' },
  { id: 'weaknesses', label: '弱点', keywords: 'weakness 薄弱', to: '/weaknesses' },
  { id: 'progress', label: '学习进度', keywords: 'progress 进度', to: '/progress' },
  { id: 'settings', label: '设置', keywords: 'settings 设置', to: '/settings' },
] as const

type Command = (typeof COMMANDS)[number]

/**
 * 命令面板（⌘K / Ctrl+K）：F1 先提供全局导航命令，后续阶段追加
 * 最近课程与动作命令。可访问性模型：焦点始终停留在输入框，
 * 用 aria-activedescendant 指向当前选项（listbox 模式）。
 * 焦点圈禁 / Esc 关闭 / 关闭后焦点回到触发器由 Radix Dialog 保证。
 */
export function CommandPalette() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [activeIndex, setActiveIndex] = useState(0)
  const navigate = useNavigate()
  const listId = useId()

  useEffect(() => {
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setOpen((value) => !value)
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return COMMANDS
    return COMMANDS.filter(
      (command) =>
        command.label.toLowerCase().includes(q) || command.keywords.toLowerCase().includes(q),
    )
  }, [query])

  const activeCommand =
    filtered.length > 0 ? filtered[Math.min(activeIndex, filtered.length - 1)] : undefined

  const runCommand = (command: Command) => {
    setOpen(false)
    void navigate({ to: command.to })
  }

  const onInputKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      if (filtered.length === 0) return
      const delta = event.key === 'ArrowDown' ? 1 : -1
      setActiveIndex((index) => (index + delta + filtered.length) % filtered.length)
    } else if (event.key === 'Enter') {
      event.preventDefault()
      if (activeCommand) runCommand(activeCommand)
    }
  }

  return (
    <Root
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) {
          setQuery('')
          setActiveIndex(0)
        }
      }}
    >
      <Trigger asChild>
        <button
          type="button"
          aria-label="打开命令面板（⌘K 或 Ctrl+K）"
          className={buttonClasses({ variant: 'secondary', size: 'sm' })}
        >
          <Search aria-hidden="true" className="h-4 w-4" />
          <span className="hidden sm:inline">命令</span>
          <kbd
            aria-hidden="true"
            className="hidden rounded border border-border bg-surface-sunken px-1 text-xs text-text-muted sm:inline"
          >
            ⌘K
          </kbd>
        </button>
      </Trigger>
      <Portal>
        <Overlay className="fixed inset-0 z-40 bg-text/40" />
        <Content
          aria-describedby={undefined}
          className={cn(
            'fixed left-1/2 top-[12dvh] z-50 w-[calc(100vw-2rem)] max-w-md -translate-x-1/2',
            'rounded-lg border border-border bg-surface shadow-dialog',
          )}
        >
          <Title className="sr-only">命令面板</Title>
          <div className="flex items-center gap-2 px-3">
            <Search aria-hidden="true" className="h-4 w-4 shrink-0 text-text-subtle" />
            <input
              role="combobox"
              aria-expanded="true"
              aria-controls={listId}
              aria-activedescendant={activeCommand ? `${listId}-${activeCommand.id}` : undefined}
              aria-label="搜索命令"
              value={query}
              onChange={(event) => {
                setQuery(event.target.value)
                setActiveIndex(0)
              }}
              onKeyDown={onInputKeyDown}
              placeholder="输入命令或搜索页面…"
              className="h-11 w-full bg-transparent text-sm text-text outline-none placeholder:text-text-subtle"
            />
          </div>
          <ul
            role="listbox"
            id={listId}
            aria-label="可用命令"
            className="max-h-72 overflow-y-auto border-t border-border p-1"
          >
            {filtered.length === 0 ? (
              <li className="px-3 py-4 text-center text-sm text-text-muted">没有匹配的命令</li>
            ) : (
              filtered.map((command, index) => {
                const isActive = command.id === activeCommand?.id
                return (
                  // combobox/listbox 模式：键盘交互全部在输入框上（方向键 +
                  // Enter + aria-activedescendant），option 只需响应指针。
                  // eslint-disable-next-line jsx-a11y/click-events-have-key-events
                  <li
                    key={command.id}
                    role="option"
                    id={`${listId}-${command.id}`}
                    aria-selected={isActive}
                    onMouseEnter={() => setActiveIndex(index)}
                    onClick={() => runCommand(command)}
                    className={cn(
                      'flex cursor-default items-center justify-between rounded-md px-3 py-2 text-sm',
                      isActive ? 'bg-accent-subtle text-accent-strong' : 'text-text',
                    )}
                  >
                    {command.label}
                  </li>
                )
              })
            )}
          </ul>
        </Content>
      </Portal>
    </Root>
  )
}
