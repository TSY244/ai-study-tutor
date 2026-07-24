import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { Weakness } from '@/api/generated/model'
import { COURSE_STATS_ID } from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { renderWithRouter } from '@/test/utils'

function weaknessShape(id: string, label: string, mastered = false): Weakness {
  return {
    id,
    course_id: COURSE_STATS_ID,
    lesson_id: null,
    concept_key: id,
    concept_label: label,
    description: `${label} 的描述`,
    evidence: [],
    severity: 3,
    review_count: 0,
    mastered,
    version: 1,
    first_seen_at: '2026-07-10T08:00:00Z',
    last_seen_at: '2026-07-15T16:00:00Z',
  }
}

describe('弱点列表', () => {
  it('展示弱点与状态徽标，按掌握状态过滤', async () => {
    const user = userEvent.setup()
    renderWithRouter('/weaknesses')

    // 全部：4 条（含 1 条已掌握）
    expect(await screen.findByText('条件概率')).toBeInTheDocument()
    expect(screen.getByText('方差计算')).toBeInTheDocument()
    expect(screen.getByText('期望的线性性')).toBeInTheDocument()
    expect(screen.getByText('线性组合与张成')).toBeInTheDocument()

    // 待复习：不含已掌握项
    await user.click(screen.getByRole('button', { name: '待复习' }))
    await waitFor(() => expect(screen.queryByText('期望的线性性')).not.toBeInTheDocument())
    expect(screen.getByText('条件概率')).toBeInTheDocument()

    // 已掌握：只剩已掌握项
    await user.click(screen.getByRole('button', { name: '已掌握' }))
    await waitFor(() => expect(screen.queryByText('条件概率')).not.toBeInTheDocument())
    expect(screen.getByText('期望的线性性')).toBeInTheDocument()
  })

  it('超过一页时按 page_info 游标加载更多', async () => {
    const user = userEvent.setup()
    server.use(
      http.get('*/api/v1/weaknesses', ({ request }) => {
        const url = new URL(request.url)
        const offset = Number(url.searchParams.get('cursor')) || 0
        const limit = Number(url.searchParams.get('limit')) || 10
        const all = Array.from({ length: 15 }, (_, index) =>
          weaknessShape(`w-${index}`, `概念 ${index + 1}`),
        )
        const items = all.slice(offset, offset + limit)
        const next = offset + limit
        return HttpResponse.json({
          items,
          page_info: {
            next_cursor: next < all.length ? String(next) : null,
            has_more: next < all.length,
          },
        })
      }),
    )

    renderWithRouter('/weaknesses')
    expect(await screen.findByText('概念 1')).toBeInTheDocument()
    expect(screen.queryByText('概念 15')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '加载更多' }))
    expect(await screen.findByText('概念 15')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '加载更多' })).not.toBeInTheDocument()
  })
})
