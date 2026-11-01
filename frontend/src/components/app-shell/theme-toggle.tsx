import { Moon, Sun } from 'lucide-react'
import { useSyncExternalStore } from 'react'

import { buttonClasses } from '@/components/ui/button-styles'
import { getTheme, subscribeTheme, toggleTheme } from '@/lib/theme'

/** 浅色/深色主题切换：常驻顶栏最右（任务中心右侧），图标随当前主题变化。 */
export function ThemeToggle() {
  const theme = useSyncExternalStore(subscribeTheme, getTheme)
  const isDark = theme === 'dark'
  const Icon = isDark ? Sun : Moon
  return (
    <button
      type="button"
      onClick={toggleTheme}
      aria-label={isDark ? '切换到浅色模式' : '切换到深色模式'}
      aria-pressed={isDark}
      className={buttonClasses({ variant: 'ghost', size: 'sm' })}
    >
      <Icon aria-hidden="true" className="h-4 w-4" />
    </button>
  )
}
