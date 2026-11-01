import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'

import { describe, expect, it } from 'vitest'

const bootstrap = readFileSync('public/theme-init.js', 'utf8')

describe('首屏主题初始化', () => {
  it.each([
    ['dark', false, true],
    ['light', true, false],
    [null, true, true],
    [null, false, false],
    ['invalid', true, true],
  ])('保存偏好 %s、系统深色 %s 时正确应用主题', (stored, systemDark, expectedDark) => {
    runInNewContext(bootstrap, {
      document,
      window: {
        localStorage: { getItem: () => stored },
        matchMedia: () => ({ matches: systemDark }),
      },
    })
    expect(document.documentElement.classList.contains('dark')).toBe(expectedDark)
  })

  it('存储不可用时仍在首屏跟随系统深色', () => {
    runInNewContext(bootstrap, {
      document,
      window: {
        get localStorage() {
          throw new Error('Storage unavailable')
        },
        matchMedia: () => ({ matches: true }),
      },
    })
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('首屏脚本通过同源外部资源加载，兼容禁止内联脚本的生产 CSP', () => {
    const html = readFileSync('index.html', 'utf8')
    const page = new DOMParser().parseFromString(html, 'text/html')
    const scripts = [...page.querySelectorAll('script')]
    expect(scripts[0].getAttribute('src')).toBe('/theme-init.js')
    expect(scripts[0].hasAttribute('defer')).toBe(false)
    expect(scripts[0].hasAttribute('async')).toBe(false)
    expect(scripts.every((script) => script.hasAttribute('src'))).toBe(true)
  })
})
