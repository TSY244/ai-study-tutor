import { fireEvent, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { COURSE_STATS_ID, LESSON_RANDOM_ID } from '@/mocks/fixtures'
import { renderWithRouter } from '@/test/utils'

/** 课时测验列表 → 开始作答 → 专注模式 的入口流程。 */
describe('课时测验页', () => {
  it('列出测验并可从「开始作答」进入专注模式', async () => {
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/quizzes`)

    // 两份测验（随堂测验 + 客观题自测）
    await screen.findByText('随机变量与分布 · 随堂测验')
    expect(screen.getByText('期望与方差 · 客观题自测')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: '开始作答' })).toHaveLength(2)

    fireEvent.click(screen.getAllByRole('button', { name: '开始作答' })[0])

    // 跳转专注模式：测验标题 + 无 App Shell 主导航 + 题目渲染
    await screen.findByRole('heading', { name: '随机变量与分布 · 随堂测验' })
    expect(screen.queryByRole('navigation', { name: '主导航' })).not.toBeInTheDocument()
    await screen.findByLabelText('作答内容')
  })
})
