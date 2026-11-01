/**
 * 浅色/深色主题：单用户本机应用，偏好持久化在 localStorage（跨标签页、跨会话生效）。
 * 未显式选择时跟随系统 prefers-color-scheme。
 *
 * 生效机制：给 <html> 加减 dark class，globals.css 的 :root.dark 覆盖全部语义色
 * token。首屏防闪烁由 public/theme-init.js 在绘制前套用同一套判定规则。
 * 初始化以 DOM 上已套用的 class 为准：正常加载时它与 存储偏好 ?? 系统 一致；
 * HMR 热替换等模块状态丢失的场景下 DOM 才是现状的唯一权威来源，可自愈不同步。
 *
 * 组件订阅用 useSyncExternalStore(subscribeTheme, getTheme)（同 job-stream-status）。
 */

export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'theme'
const listeners = new Set<() => void>()

let current: Theme | null = null
// undefined 表示尚未初始化；null 表示跟随系统。保留内存偏好以兼容存储失败。
let preference: Theme | null | undefined

function systemTheme(): Theme {
  return typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-color-scheme: dark)').matches
    ? 'dark'
    : 'light'
}

function readStoredTheme(): Theme | null {
  try {
    const stored = globalThis.localStorage?.getItem(STORAGE_KEY)
    return stored === 'light' || stored === 'dark' ? stored : null
  } catch {
    return null
  }
}

/** DOM 现状：theme-init.js 首屏套用的 dark class。 */
function readDomTheme(): Theme | null {
  if (typeof document === 'undefined') return null
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}

function applyTheme(theme: Theme): void {
  document.documentElement.classList.toggle('dark', theme === 'dark')
}

/** 当前主题：显式偏好优先，其次 DOM 首屏结果，最后跟随系统。 */
export function getTheme(): Theme {
  if (current === null) {
    preference = readStoredTheme()
    current = preference ?? readDomTheme() ?? systemTheme()
    // 首屏脚本未生效时也要校正 DOM，避免按钮状态与页面颜色相反。
    applyTheme(current)
  }
  return current
}

/** 设定主题：更新 DOM、持久化，并通知订阅者。 */
export function setTheme(theme: Theme): void {
  preference = theme
  current = theme
  applyTheme(theme)
  try {
    globalThis.localStorage?.setItem(STORAGE_KEY, theme)
  } catch {
    // 写入失败（隐私模式/配额）时退化为本会话内存态，不阻塞切换。
  }
  for (const listener of listeners) listener()
}

export function toggleTheme(): void {
  setTheme(getTheme() === 'dark' ? 'light' : 'dark')
}

export function subscribeTheme(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** 接收其他标签页的 localStorage 变化；不回写，避免覆盖来源页的偏好。 */
function onThemeStorage(event: StorageEvent): void {
  if (event.key !== STORAGE_KEY && event.key !== null) return
  if (event.storageArea !== null && event.storageArea !== globalThis.localStorage) return
  preference = readStoredTheme()
  current = preference ?? systemTheme()
  applyTheme(current)
  for (const listener of listeners) listener()
}

/** 仅自动模式响应系统主题变化，手动选择（包括存储失败的内存偏好）始终优先。 */
function onSystemThemeChange(event: MediaQueryListEvent): void {
  getTheme()
  if (preference !== null) return
  current = event.matches ? 'dark' : 'light'
  applyTheme(current)
  for (const listener of listeners) listener()
}

// 在专注作答等没有 ThemeToggle 的页面也保持同步；HMR 时移除旧模块的监听。
if (typeof window !== 'undefined') {
  // 专注作答页没有主题按钮，也应在应用入口加载时恢复主题。
  getTheme()
  const systemPreference =
    typeof window.matchMedia === 'function'
      ? window.matchMedia('(prefers-color-scheme: dark)')
      : undefined
  window.addEventListener('storage', onThemeStorage)
  systemPreference?.addEventListener('change', onSystemThemeChange)
  import.meta.hot?.dispose(() => {
    window.removeEventListener('storage', onThemeStorage)
    systemPreference?.removeEventListener('change', onSystemThemeChange)
  })
}

/** 测试隔离用：清除持久化偏好与内存态，还原为浅色。 */
export function resetTheme(): void {
  current = null
  preference = undefined
  applyTheme('light')
  try {
    globalThis.localStorage?.removeItem(STORAGE_KEY)
  } catch {
    // 与写入同理，清除失败不阻塞。
  }
}
