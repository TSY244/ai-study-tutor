import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BookOpen } from 'lucide-react'
import { describe, expect, it, vi } from 'vitest'

import { ApiProblemError } from '@/api/problem'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'

describe('ErrorState', () => {
  it('Problem 按 code 映射稳定文案 + 错误码 + 请求 ID，role=alert（不看 detail）', () => {
    const error = new ApiProblemError({
      status: 409,
      code: 'version_conflict',
      title: '版本冲突',
      detail: '服务端自然语言 detail 不参与前端文案。',
      requestId: 'req-abc-1',
    })
    render(<ErrorState error={error} onRetry={() => {}} />)

    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('请查看当前版本，合并你的修改后重试。')
    expect(alert).not.toHaveTextContent('服务端自然语言 detail')
    expect(alert).toHaveTextContent('version_conflict')
    expect(alert).toHaveTextContent('req-abc-1')
  })

  it('非 Problem 错误显示兜底文案；重试按钮触发回调', async () => {
    const user = userEvent.setup()
    const onRetry = vi.fn()
    render(<ErrorState error={new Error('boom')} onRetry={onRetry} />)

    expect(screen.getByRole('alert')).toHaveTextContent('发生了未预期的错误。')
    await user.click(screen.getByRole('button', { name: '重试' }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })
})

describe('EmptyState', () => {
  it('渲染标题、原因与唯一下一步操作', () => {
    render(
      <EmptyState
        icon={BookOpen}
        title="还没有课程"
        description="创建第一门课程开始学习。"
        action={<button type="button">新建课程</button>}
      />,
    )
    expect(screen.getByText('还没有课程')).toBeInTheDocument()
    expect(screen.getByText('创建第一门课程开始学习。')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '新建课程' })).toBeInTheDocument()
  })
})
