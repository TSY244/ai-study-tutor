import { Component, useEffect, useRef, type ErrorInfo, type ReactNode } from 'react'

import { isApiProblemError } from '@/api/problem'

/**
 * 全局 Error Boundary：渲染期错误的最后防线。
 * 展示可行动的恢复入口；若是 API 错误，暴露 X-Request-ID 便于诊断。
 * 路由级错误由 TanStack Router 的 errorComponent 复用同一展示组件。
 */

interface ErrorBoundaryProps {
  children: ReactNode
  /** 可选：自定义兜底展示；默认使用 ErrorFallback。 */
  fallback?: (error: unknown, reset: () => void) => ReactNode
  onError?: (error: unknown, info?: ErrorInfo) => void
}

interface ErrorBoundaryState {
  error: unknown
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  override state: ErrorBoundaryState = { error: undefined }

  static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    return { error }
  }

  override componentDidCatch(error: unknown, info: ErrorInfo): void {
    this.props.onError?.(error, info)
  }

  private reset = () => this.setState({ error: undefined })

  override render(): ReactNode {
    if (this.state.error !== undefined) {
      if (this.props.fallback) return this.props.fallback(this.state.error, this.reset)
      return <ErrorFallback error={this.state.error} onReset={this.reset} />
    }
    return this.props.children
  }
}

interface ErrorFallbackProps {
  error: unknown
  onReset?: () => void
}

export function ErrorFallback({ error, onReset }: ErrorFallbackProps) {
  const problem = isApiProblemError(error) ? error : undefined
  const heading = problem ? problem.title : '页面出现错误'
  const detail = problem
    ? problem.detail
    : '发生了未预期的界面错误。你的数据保存在本地服务中，可以安全重试。'
  // 错误发生后焦点归属到标题，屏幕阅读器与键盘用户有明确落点
  const headingRef = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    headingRef.current?.focus()
  }, [])

  return (
    <div role="alert" className="mx-auto max-w-lg px-4 py-16">
      <h1 tabIndex={-1} ref={headingRef} className="text-lg text-text outline-none">
        {heading}
      </h1>
      <p className="mt-2 text-text-muted">{detail}</p>
      {problem ? (
        <dl className="mt-4 space-y-1 text-sm text-text-subtle">
          <div className="flex gap-2">
            <dt className="font-medium">错误码</dt>
            <dd className="font-mono">{problem.code}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="font-medium">请求 ID</dt>
            <dd className="font-mono break-all">{problem.requestId}</dd>
          </div>
        </dl>
      ) : null}
      <div className="mt-6 flex items-center gap-3">
        {onReset ? (
          <button
            type="button"
            onClick={onReset}
            className="rounded-md bg-accent px-3 py-2 font-medium text-accent-contrast hover:bg-accent-strong"
          >
            重试
          </button>
        ) : null}
        <a href="/dashboard" className="font-medium text-accent underline-offset-2 hover:underline">
          返回学习总览
        </a>
      </div>
    </div>
  )
}
