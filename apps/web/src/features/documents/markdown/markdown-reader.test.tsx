import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { MarkdownReader } from './markdown-reader'

describe('MarkdownReader 安全渲染', () => {
  it('不渲染原始 HTML/脚本/事件属性，危险协议链接被剥掉', () => {
    const markdown = [
      '# 正常标题',
      '<script>window.__pwned = true</script>',
      '<img src="x" onerror="window.__pwned = true">',
      '<iframe src="https://evil.example"></iframe>',
      '[危险链接](javascript:window.__pwned=true)',
      '<a href="https://ok.example" onclick="alert(1)">原始 HTML 链接</a>',
    ].join('\n\n')

    const { container } = render(<MarkdownReader markdown={markdown} />)

    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('iframe')).toBeNull()
    // Markdown 图片语法正常渲染，但绝不带事件处理器属性
    expect(container.querySelector('[onerror], [onclick], [onload]')).toBeNull()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()

    // javascript: 协议被剥掉：锚点没有可导航的 href（退化为不可点击文本）
    const dangerText = screen.getByText('危险链接')
    const anchor = dangerText.closest('a')
    expect(anchor?.getAttribute('href')?.startsWith('javascript:')).toBeFalsy()

    expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined()
  })

  it('Markdown 图片语法渲染懒加载 img，正常链接保留', () => {
    render(
      <MarkdownReader
        markdown={'![示意图](/files/demo.png)\n\n[参考资料](https://example.com/doc)'}
      />,
    )
    const img = screen.getByRole('img', { name: '示意图' })
    expect(img).toHaveAttribute('src', '/files/demo.png')
    expect(img).toHaveAttribute('loading', 'lazy')
    expect(screen.getByRole('link', { name: '参考资料' })).toHaveAttribute(
      'href',
      'https://example.com/doc',
    )
  })
})

describe('MarkdownReader 锚点与目录', () => {
  it('标题生成稳定 id，重复标题自动去重，目录链接与之一致', () => {
    const markdown = ['## 随机变量', '第一段。', '## 随机变量', '第二段。', '### 离散型'].join(
      '\n\n',
    )
    render(<MarkdownReader markdown={markdown} />)

    const headings = screen.getAllByRole('heading')
    const ids = headings.map((heading) => heading.id)
    expect(ids).toEqual(['随机变量', '随机变量-1', '离散型'])
    // 目录跳转后可聚焦（键盘/屏幕阅读器继续阅读）
    for (const heading of headings) expect(heading).toHaveAttribute('tabindex', '-1')

    const tocLinks = screen
      .getAllByRole('link', { name: '随机变量' })
      .map((link) => link.getAttribute('href'))
    expect(tocLinks).toContain('#随机变量')
    expect(tocLinks).toContain('#随机变量-1')
  })

  it('代码围栏内的 # 行不会进入目录', () => {
    const markdown = ['## 真正标题', '```', '# 这是注释不是标题', '```'].join('\n')
    render(<MarkdownReader markdown={markdown} />)

    expect(screen.getAllByRole('heading')).toHaveLength(1)
    expect(screen.queryByRole('link', { name: '这是注释不是标题' })).not.toBeInTheDocument()
  })
})

describe('MarkdownReader 代码块', () => {
  it('复制按钮把代码写入剪贴板并给出确认', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    })

    render(<MarkdownReader markdown={'```python\nprint("hello")\n```'} />)
    fireEvent.click(screen.getByRole('button', { name: /复制/ }))

    await waitFor(() => expect(writeText).toHaveBeenCalledWith('print("hello")'))
    expect(await screen.findByRole('button', { name: /已复制/ })).toBeInTheDocument()
  })

  it('已知语言异步加载 shiki 高亮，初始渲染为纯文本', async () => {
    const { container } = render(<MarkdownReader markdown={'```python\nprint("hi")\n```'} />)
    // 初始为纯文本（高亮不阻塞首屏/长文渲染）
    expect(container.querySelector('.shiki')).toBeNull()
    await waitFor(() => expect(container.querySelector('.shiki')).not.toBeNull(), {
      timeout: 15_000,
    })
    expect(screen.getByText(/print/)).toBeInTheDocument()
  }, 20_000)

  it('未知语言退回纯文本，不尝试高亮', async () => {
    const { container } = render(<MarkdownReader markdown={'```cobol\nDISPLAY "HI".\n```'} />)
    await new Promise((resolve) => setTimeout(resolve, 300))
    expect(container.querySelector('.shiki')).toBeNull()
    expect(screen.getByText('DISPLAY "HI".')).toBeInTheDocument()
  })
})

describe('MarkdownReader 数学公式', () => {
  it('合法公式渲染为 KaTeX', () => {
    const { container } = render(<MarkdownReader markdown={'质能方程 $E = mc^2$ 成立。'} />)
    expect(container.querySelector('.katex')).not.toBeNull()
  })

  it('非法公式降级显示原始表达式，不抛出错误', () => {
    const { container } = render(<MarkdownReader markdown={'坏公式 $\\frac{1}{$ 在此。'} />)
    expect(container.querySelector('.katex-error')).not.toBeNull()
    expect(container).toHaveTextContent('\\frac{1}{')
  })
})

describe('MarkdownReader 长文性能', () => {
  it('长文档一次渲染完成，标题锚点齐全，高亮不阻塞渲染', () => {
    const sections: string[] = []
    for (let i = 0; i < 200; i += 1) {
      sections.push(
        `## 第 ${i + 1} 节\n\n这是第 ${i + 1} 节的正文，包含若干学习内容，覆盖概念、示例与练习要点。`,
      )
      if (i % 5 === 0) {
        sections.push(`\`\`\`python\nvalue_${i} = ${i}\nprint(value_${i})\n\`\`\``)
      }
    }
    const markdown = sections.join('\n\n')
    expect(markdown.length).toBeGreaterThan(11_000)

    const start = performance.now()
    render(<MarkdownReader markdown={markdown} />)
    const elapsed = performance.now() - start

    // 预算宽松（CI 抖动），真实回归（如同步高亮）会超出一个数量级
    expect(elapsed).toBeLessThan(5_000)
    const headings = screen.getAllByRole('heading', { level: 2 })
    expect(headings).toHaveLength(200)
    for (const heading of headings) expect(heading.id).not.toBe('')
  }, 15_000)
})
