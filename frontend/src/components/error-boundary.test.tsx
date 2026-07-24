import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ApiProblemError } from '@/api/problem'
import { ErrorBoundary } from '@/components/error-boundary'

function ThrowingChild({ error }: { error: unknown }): never {
  throw error
}

afterEach(() => vi.restoreAllMocks())

describe('ErrorBoundary', () => {
  it('捕获渲染错误，展示兜底并把焦点归属到标题', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(
      <ErrorBoundary>
        <ThrowingChild error={new Error('boom')} />
      </ErrorBoundary>,
    )

    const heading = screen.getByRole('heading', { name: '页面出现错误' })
    expect(heading).toBeInTheDocument()
    expect(heading).toHaveFocus()
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })

  it('API 错误展示错误码与 X-Request-ID 供诊断', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const problem = new ApiProblemError({
      status: 503,
      code: 'claude_unavailable',
      title: '服务暂不可用',
      detail: 'Claude 暂时不可用，请稍后重试。',
      requestId: 'req-diag-1',
    })
    render(
      <ErrorBoundary>
        <ThrowingChild error={problem} />
      </ErrorBoundary>,
    )

    expect(screen.getByText('claude_unavailable')).toBeInTheDocument()
    expect(screen.getByText('req-diag-1')).toBeInTheDocument()
  })

  it('点击重试后恢复渲染子树', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const user = userEvent.setup()
    let shouldThrow = true
    function MaybeThrowing() {
      if (shouldThrow) throw new Error('boom')
      return <p>已恢复</p>
    }
    render(
      <ErrorBoundary>
        <MaybeThrowing />
      </ErrorBoundary>,
    )
    expect(screen.getByRole('alert')).toBeInTheDocument()

    shouldThrow = false
    await user.click(screen.getByRole('button', { name: '重试' }))
    expect(screen.getByText('已恢复')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
