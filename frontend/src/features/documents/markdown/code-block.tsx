import { Check, Copy } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import { highlightCode } from './highlight'

/**
 * 代码块：复制按钮 + 渐进式语法高亮。
 * 初始渲染始终是纯文本 <pre>（长文性能：首屏不做高亮），shiki 加载完成后
 * 原地替换为高亮 HTML；失败则保持纯文本。
 */
export function CodeBlock({ language, code }: { language: string | null; code: string }) {
  const [highlighted, setHighlighted] = useState<{ code: string; html: string } | null>(null)
  const [copied, setCopied] = useState(false)
  const timerRef = useRef<number | null>(null)

  useEffect(() => {
    if (!language) return
    let cancelled = false
    void highlightCode(code, language).then((html) => {
      if (!cancelled && html) setHighlighted({ code, html })
    })
    return () => {
      cancelled = true
    }
  }, [code, language])

  useEffect(
    () => () => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current)
    },
    [],
  )

  async function copy() {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      if (timerRef.current !== null) window.clearTimeout(timerRef.current)
      timerRef.current = window.setTimeout(() => setCopied(false), 2000)
    } catch {
      // 剪贴板不可用（权限/非安全上下文）：不打扰阅读，按钮可重试。
    }
  }

  // 仅当高亮结果与当前代码对应时才使用，避免代码切换后闪现旧高亮。
  const html = highlighted?.code === code ? highlighted.html : null

  return (
    <div className="md-codeblock">
      <button
        type="button"
        onClick={() => void copy()}
        className="absolute right-2 top-2 inline-flex items-center gap-1 rounded-sm border border-border bg-surface px-1.5 py-0.5 text-xs text-text-muted hover:text-text"
      >
        {copied ? (
          <Check aria-hidden="true" className="h-3.5 w-3.5 text-ok" />
        ) : (
          <Copy aria-hidden="true" className="h-3.5 w-3.5" />
        )}
        {copied ? '已复制' : '复制'}
      </button>
      {html ? (
        // shiki 输出（语法 token 的 span 序列），由本地代码文本生成，不含输入 HTML。
        <div dangerouslySetInnerHTML={{ __html: html }} />
      ) : (
        <pre>
          <code>{code}</code>
        </pre>
      )}
    </div>
  )
}
