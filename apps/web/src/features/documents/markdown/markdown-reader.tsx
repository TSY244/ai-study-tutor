import { isValidElement, useEffect, useMemo, type ReactNode } from 'react'
import ReactMarkdown, { type Components } from 'react-markdown'
import rehypeKatex from 'rehype-katex'
import rehypeSlug from 'rehype-slug'
import katexStyles from 'katex/dist/katex.min.css?inline'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'

import { CodeBlock } from './code-block'
import { extractToc } from './heading-utils'
import { MarkdownToc } from './markdown-toc'
import { nodeText } from './node-text'

/**
 * 安全 Markdown 阅读器（02-frontend-handoff §6.3）：
 * - 不使用 rehype-raw：Markdown 中的原始 HTML/脚本/iframe 不会被渲染；
 *   URL 走 react-markdown 默认 urlTransform，javascript: 等协议被剥掉。
 * - 标题由 rehype-slug 写入稳定 id，目录（heading-utils）用同一算法生成锚点。
 * - 数学公式走 KaTeX；throwOnError: false，解析失败时原地显示原始表达式。
 * - 代码块高亮在 code-block 内懒加载；本模块（react-markdown + KaTeX）只被
 *   阅读路由按需加载，不进首屏 bundle。
 */

const components: Components = {
  pre({ node: _node, children }) {
    const codeElement = isValidElement<{ className?: string; children?: ReactNode }>(children)
      ? children
      : null
    if (!codeElement) return <pre>{children}</pre>
    const language =
      /language-([\w+-]+)/.exec(codeElement.props.className ?? '')?.[1]?.toLowerCase() ?? null
    // react-markdown 给块级代码补结尾换行，复制时不带出去。
    const code = nodeText(codeElement.props.children).replace(/\n$/, '')
    return <CodeBlock language={language} code={code} />
  },
  // tabIndex=-1：目录锚点跳转后焦点可以落在标题上（键盘/屏幕阅读器继续读）。
  h1: ({ node: _node, children, ...props }) => (
    <h1 tabIndex={-1} {...props}>
      {children}
    </h1>
  ),
  h2: ({ node: _node, children, ...props }) => (
    <h2 tabIndex={-1} {...props}>
      {children}
    </h2>
  ),
  h3: ({ node: _node, children, ...props }) => (
    <h3 tabIndex={-1} {...props}>
      {children}
    </h3>
  ),
  h4: ({ node: _node, children, ...props }) => (
    <h4 tabIndex={-1} {...props}>
      {children}
    </h4>
  ),
  a: ({ node: _node, children, ...props }) => (
    <a {...props} rel="noreferrer">
      {children}
    </a>
  ),
  img: ({ node: _node, ...props }) => <img loading="lazy" {...props} alt={props.alt ?? ''} />,
  table: ({ node: _node, ...props }) => (
    <div className="md-table-scroll">
      <table {...props} />
    </div>
  ),
}

const KATEX_STYLE_ID = 'katex-inline-styles'

export function MarkdownReader({ markdown }: { markdown: string }) {
  useEffect(() => {
    if (document.getElementById(KATEX_STYLE_ID)) return
    const style = document.createElement('style')
    style.id = KATEX_STYLE_ID
    style.textContent = katexStyles
    document.head.append(style)
  }, [])

  const toc = useMemo(() => extractToc(markdown), [markdown])
  // 解析/渲染只在文档内容变化时重做：更新提示、历史版本切换等状态变化
  // 不会触发整篇重解析（长文性能）。
  const rendered = useMemo(
    () => (
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[
          rehypeSlug,
          [rehypeKatex, { strict: false, throwOnError: false, errorColor: '#b91c1c' }],
        ]}
        components={components}
      >
        {markdown}
      </ReactMarkdown>
    ),
    [markdown],
  )

  return (
    <div className="space-y-3 xl:flex xl:items-start xl:gap-8 xl:space-y-0">
      {toc.length > 0 ? <MarkdownToc headings={toc} variant="mobile" /> : null}
      <article aria-label="文档内容" className="md-body min-w-0 flex-1">
        {rendered}
      </article>
      {toc.length > 0 ? <MarkdownToc headings={toc} variant="desktop" /> : null}
    </div>
  )
}
