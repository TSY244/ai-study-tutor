import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getTheme, resetTheme, setTheme, subscribeTheme, toggleTheme } from '@/lib/theme'

beforeEach(() => {
  resetTheme()
})

describe('getTheme 初始化', () => {
  it('无存储偏好且 DOM 无 dark class 时默认为浅色', () => {
    expect(getTheme()).toBe('light')
  })

  it('存储偏好优先于 DOM 现状', () => {
    document.documentElement.classList.add('dark')
    localStorage.setItem('theme', 'light')
    expect(getTheme()).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('已保存深色但 DOM 为浅色时先校正页面，首次点击即可切换', () => {
    localStorage.setItem('theme', 'dark')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(getTheme()).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)

    toggleTheme()
    expect(getTheme()).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('DOM 已有 dark class 但无存储偏好时以 DOM 为准（HMR 不同步自愈）', () => {
    // 模拟模块状态被热替换重置、但 <html> 仍保留首屏内联脚本结果的现场：
    // 若此时按系统主题推算出与 DOM 相反的值，首次点击会方向相反而看似“没反应”。
    document.documentElement.classList.add('dark')
    expect(getTheme()).toBe('dark')

    // 首次切换必须是 dark -> light，而不是再来一次 dark
    toggleTheme()
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(localStorage.getItem('theme')).toBe('light')
  })
})

describe('toggleTheme', () => {
  it('浅色初始下一次切换到深色并持久化', () => {
    toggleTheme()
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')

    toggleTheme()
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(localStorage.getItem('theme')).toBe('light')
  })

  it('setTheme 后 getTheme 与 DOM 保持一致', () => {
    setTheme('dark')
    expect(getTheme()).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })
})

describe('跨标签页主题同步', () => {
  it('同步其他标签页的偏好并在 DOM 更新后通知订阅者', () => {
    setTheme('light')
    const snapshots: string[] = []
    const unsubscribe = subscribeTheme(() => {
      snapshots.push(`${getTheme()}:${document.documentElement.classList.contains('dark')}`)
    })
    try {
      localStorage.setItem('theme', 'dark')
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: 'theme',
          newValue: 'dark',
          storageArea: localStorage,
        }),
      )
      expect(getTheme()).toBe('dark')
      expect(snapshots).toEqual(['dark:true'])
    } finally {
      unsubscribe()
    }
  })

  it.each(['theme', null])('其他标签页清除偏好（key=%s）后恢复系统主题', (key) => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    setTheme('light')
    localStorage.removeItem('theme')
    window.dispatchEvent(new StorageEvent('storage', { key, storageArea: localStorage }))
    expect(getTheme()).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBeNull()
  })

  it('其他存储项与 sessionStorage 事件不改变主题', () => {
    setTheme('light')
    localStorage.setItem('theme', 'dark')
    window.dispatchEvent(new StorageEvent('storage', { key: 'other', storageArea: localStorage }))
    window.dispatchEvent(
      new StorageEvent('storage', { key: 'theme', newValue: 'dark', storageArea: sessionStorage }),
    )
    expect(getTheme()).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })
})
