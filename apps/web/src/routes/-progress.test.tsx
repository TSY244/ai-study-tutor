import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { renderWithRouter } from '@/test/utils'

describe('学习进度页', () => {
  it('展示课程掌握度、趋势与统计；单点课程显示趋势占位', async () => {
    renderWithRouter('/progress')

    // 统计学基础：两个趋势点 -> SVG 趋势图（aria 摘要含起止百分比与次数）
    const statsCard = await screen.findByRole('region', { name: '统计学基础 进度' })
    expect(
      await within(statsCard).findByRole('img', {
        name: '统计学基础掌握度从 45% 变化到 62%，共 2 次记录',
      }),
    ).toBeInTheDocument()
    expect(within(statsCard).getByText('62%')).toBeInTheDocument()

    // 线性代数：只有一个趋势点 -> 趋势占位文案，不硬画一条假趋势
    const linalgCard = await screen.findByRole('region', { name: '线性代数 进度' })
    expect(
      within(linalgCard).getByText('完成更多测验后，这里会显示掌握度趋势。'),
    ).toBeInTheDocument()
  })
})
