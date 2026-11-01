import { act, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { Document } from '@/api/generated/model'
import {
  COURSE_STATS_ID,
  JOB_SUCCEEDED_ID,
  LESSON_BAYES_ID,
  LESSON_CONFIDENCE_ID,
  LESSON_RANDOM_ID,
} from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { renderWithRouter } from '@/test/utils'

const NOTES_PATH = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/notes`

describe('文档阅读视图', () => {
  it('渲染当前笔记：Markdown 内容、版本徽标与目录', async () => {
    renderWithRouter(NOTES_PATH)

    const article = await screen.findByRole('article', { name: '文档内容' })
    expect(
      within(article).getByRole('heading', { level: 1, name: '随机变量与分布' }),
    ).toBeInTheDocument()
    expect(screen.getByText('R3')).toBeInTheDocument()

    // 目录（桌面/窄屏两种形态至少一种在文档中）
    expect(screen.getAllByRole('navigation', { name: '文档目录' }).length).toBeGreaterThan(0)
    // 代码块带复制按钮；数学公式经 KaTeX 渲染
    expect(within(article).getAllByRole('button', { name: /复制/ }).length).toBeGreaterThan(0)
    expect(article.querySelector('.katex')).not.toBeNull()
  })

  it('大纲子视图渲染教师大纲文档', async () => {
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/outline`)
    const article = await screen.findByRole('article', { name: '文档内容' })
    expect(within(article).getByRole('heading', { level: 1 })).toHaveTextContent(
      '随机变量与分布 · 教学大纲',
    )
  })

  it('原文子视图渲染导入原文文档', async () => {
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/raw`)
    const article = await screen.findByRole('article', { name: '文档内容' })
    expect(within(article).getByRole('heading', { level: 1 })).toHaveTextContent(
      '导入原文：随机变量与分布',
    )
  })

  it('历史版本：列表展示、查看只读快照并回到当前版本', async () => {
    const user = userEvent.setup()
    renderWithRouter(NOTES_PATH)
    await screen.findByRole('article', { name: '文档内容' })

    await user.click(screen.getByRole('button', { name: /历史版本/ }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('R3')).toBeInTheDocument()
    expect(within(dialog).getByText('当前版本')).toBeInTheDocument()
    expect(within(dialog).getByText('R2')).toBeInTheDocument()
    expect(within(dialog).getByText('R1')).toBeInTheDocument()
    // 共 3 个版本，不足一页，不显示"加载更多"
    expect(within(dialog).queryByRole('button', { name: '加载更多' })).not.toBeInTheDocument()

    const r1Row = within(dialog).getByText('R1').closest('li') as HTMLElement
    await user.click(within(r1Row).getByRole('button', { name: '查看' }))

    // 快照提示 + 内容切换为 R1（只有一节的早期版本），不打断当前页面
    expect(await screen.findByText(/正在查看历史版本 R1/)).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    const article = screen.getByRole('article', { name: '文档内容' })
    expect(within(article).queryByRole('heading', { name: '期望与方差' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '回到当前版本' }))
    expect(screen.queryByText(/正在查看历史版本/)).not.toBeInTheDocument()
    expect(
      within(screen.getByRole('article', { name: '文档内容' })).getByRole('heading', {
        name: '期望与方差',
      }),
    ).toBeInTheDocument()
  })

  it('历史版本超过一页时按 page_info 游标加载更多', async () => {
    const user = userEvent.setup()
    server.use(
      http.get('*/api/v1/lessons/:lessonId/documents/:kind/versions', ({ request }) => {
        const url = new URL(request.url)
        const offset = Number(url.searchParams.get('cursor')) || 0
        const limit = Number(url.searchParams.get('limit')) || 10
        const all = Array.from({ length: 12 }, (_, index) => ({
          id: `00000000-0000-4000-8000-0000000000${String(100 + index)}`,
          lesson_id: LESSON_RANDOM_ID,
          kind: 'student_notes',
          revision: 12 - index,
          source_job_id: null,
          is_active: index === 0,
          created_at: '2026-07-16T09:52:00Z',
        }))
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

    renderWithRouter(NOTES_PATH)
    await screen.findByRole('article', { name: '文档内容' })
    await user.click(screen.getByRole('button', { name: /历史版本/ }))
    const dialog = await screen.findByRole('dialog', {}, { timeout: 5000 })

    expect(within(dialog).getByText('R12')).toBeInTheDocument()
    expect(within(dialog).queryByText('R1')).not.toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: '加载更多' }))
    expect(await within(dialog).findByText('R1')).toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: '加载更多' })).not.toBeInTheDocument()
  })

  it('内容更新只提示不打断：确认后才切换到新版本', async () => {
    const user = userEvent.setup()
    const { client } = renderWithRouter(NOTES_PATH)
    const article = await screen.findByRole('article', { name: '文档内容' })
    expect(
      within(article).getByRole('heading', { level: 1, name: '随机变量与分布' }),
    ).toBeInTheDocument()

    const newDoc: Document = {
      id: 'f0000000-0000-4000-8000-0000000000ff',
      lesson_id: LESSON_RANDOM_ID,
      kind: 'student_notes',
      revision: 4,
      content_md: '# 随机变量与分布（第四版）\n\n## 新增章节\n\n新补充的内容。',
      source_job_id: JOB_SUCCEEDED_ID,
      is_active: true,
      created_at: '2026-07-18T09:00:00Z',
    }
    server.use(
      http.get('*/api/v1/lessons/:lessonId/documents/student_notes', () =>
        HttpResponse.json(newDoc),
      ),
    )

    // 模拟生成任务完成后的精确失效触发后台 refetch
    await act(async () => {
      await client.invalidateQueries({ queryKey: ['document', LESSON_RANDOM_ID] })
    })

    // 出现更新提示，但阅读内容仍是旧版本（不打断阅读）
    expect(await screen.findByText(/内容已更新/)).toBeInTheDocument()
    expect(screen.getByText('R3')).toBeInTheDocument()
    expect(
      within(screen.getByRole('article', { name: '文档内容' })).queryByRole('heading', {
        name: '新增章节',
      }),
    ).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '查看新版本' }))
    expect(await screen.findByText('R4')).toBeInTheDocument()
    expect(
      within(screen.getByRole('article', { name: '文档内容' })).getByRole('heading', {
        name: '新增章节',
      }),
    ).toBeInTheDocument()
    expect(screen.queryByText(/内容已更新/)).not.toBeInTheDocument()
  })

  it('更新提示可忽略：继续阅读当前版本', async () => {
    const user = userEvent.setup()
    const { client } = renderWithRouter(NOTES_PATH)
    await screen.findByRole('article', { name: '文档内容' })

    const newDoc: Document = {
      id: 'f0000000-0000-4000-8000-0000000000fe',
      lesson_id: LESSON_RANDOM_ID,
      kind: 'student_notes',
      revision: 4,
      content_md: '# 随机变量与分布（第四版）',
      source_job_id: JOB_SUCCEEDED_ID,
      is_active: true,
      created_at: '2026-07-18T09:00:00Z',
    }
    server.use(
      http.get('*/api/v1/lessons/:lessonId/documents/student_notes', () =>
        HttpResponse.json(newDoc),
      ),
    )
    await act(async () => {
      await client.invalidateQueries({ queryKey: ['document', LESSON_RANDOM_ID] })
    })
    expect(await screen.findByText(/内容已更新/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '继续阅读当前版本' }))
    expect(screen.queryByText(/内容已更新/)).not.toBeInTheDocument()
    expect(screen.getByText('R3')).toBeInTheDocument()
  })

  it('笔记未生成时展示空状态与下一步引导', async () => {
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_CONFIDENCE_ID}/notes`)
    expect(await screen.findByText('笔记尚未生成')).toBeInTheDocument()
    expect(screen.getByText(/导入素材并等待生成任务完成后/)).toBeInTheDocument()
  })

  it('导入进行中时，在笔记下显示任务进度而非尚未生成', async () => {
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_BAYES_ID}/notes`)
    const jobs = await screen.findByRole('region', { name: '后台生成任务' })
    expect(within(jobs).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '45')
    expect(within(jobs).getByRole('link', { name: '查看任务详情' })).toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: '查看任务详情' })).toHaveLength(1)
    const tabs = screen.getByRole('navigation', { name: '课时子视图' })
    expect(tabs.compareDocumentPosition(jobs) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(screen.queryByText('笔记尚未生成')).not.toBeInTheDocument()
  })
})
