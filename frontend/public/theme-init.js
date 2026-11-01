/* global window, document */

// 在首屏绘制前应用主题；同源外部脚本可在生产 CSP 下正常执行。
// 与 src/lib/theme.ts 使用同一优先级：保存的偏好优先，否则跟随系统。
;(() => {
  let theme = null
  try {
    theme = window.localStorage.getItem('theme')
  } catch {
    // 存储不可用时继续跟随系统。
  }
  if (theme !== 'light' && theme !== 'dark') {
    theme =
      typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-color-scheme: dark)').matches
        ? 'dark'
        : 'light'
  }
  document.documentElement.classList.toggle('dark', theme === 'dark')
})()
