import { screen, waitFor, within } from '@testing-library/react'
import { http } from 'msw'
import { describe, expect, it } from 'vitest'

import { loadRecentLearning, recordRecentLearning } from '@/features/learning/recent-learning'
import { COURSE_STATS_ID, LESSON_RANDOM_ID } from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { problemResponse } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

describe('学习总览', () => {
  it('展示课程、进行中任务与系统状态（链路：路由 -> Query -> 生成 Client -> MSW）', async () => {
    renderWithRouter('/dashboard')

    expect(await screen.findByRole('heading', { name: '学习总览' })).toBeInTheDocument()
    // 我的课程（curated fixture）
    expect(await screen.findByRole('link', { name: /统计学基础/ })).toBeInTheDocument()
    // 进行中的任务（fixture：lesson_import running 45%）
    expect(await screen.findByText('生成学生笔记')).toBeInTheDocument()
    // 系统状态
    expect(await screen.findByText('版本 0.3.0-draft')).toBeInTheDocument()
    expect(screen.getByText('Claude 连接')).toBeInTheDocument()
    // 主导航当前页状态
    const nav = screen.getByRole('navigation', { name: '主导航' })
    expect(within(nav).getByRole('link', { name: '学习总览' })).toHaveAttribute(
      'aria-current',
      'page',
    )
  })

  it('Problem 错误展示错误码、请求 ID 与重试入口，不清空页面结构', async () => {
    server.use(
      http.get('*/api/v1/system/status', () =>
        problemResponse({
          status: 503,
          code: 'maintenance_mode',
          title: '维护中',
          detail: '系统正在执行一致性备份，暂时拒绝写入。',
        }),
      ),
    )
    renderWithRouter('/dashboard')

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByText('maintenance_mode')).toBeInTheDocument()
    expect(screen.getByText('req-test-1')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '重试' })).toBeInTheDocument()
    // 页面骨架仍在，不被错误清空
    expect(screen.getByRole('heading', { name: '学习总览' })).toBeInTheDocument()
  })

  it('无学习记录时「继续学习」展示空状态', async () => {
    renderWithRouter('/dashboard')

    expect(await screen.findByText('还没有学习记录')).toBeInTheDocument()
  })

  it('有学习记录时「继续学习」展示最近课时并链回课时工作区', async () => {
    recordRecentLearning({
      courseId: COURSE_STATS_ID,
      lessonId: LESSON_RANDOM_ID,
      lessonTitle: '随机变量与分布',
    })
    renderWithRouter('/dashboard')

    const link = await screen.findByRole('link', { name: /随机变量与分布/ })
    expect(link).toHaveAttribute('href', `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}`)
    expect(screen.getByText(/上次学习/)).toBeInTheDocument()
    expect(screen.queryByText('还没有学习记录')).not.toBeInTheDocument()
  })

  it('「继续学习」中已删除课时的记录被隐藏并从本地清理', async () => {
    recordRecentLearning({
      courseId: COURSE_STATS_ID,
      lessonId: 'lesson-already-deleted',
      lessonTitle: '已删除的课时',
    })
    server.use(
      http.get('*/api/v1/lessons/lesson-already-deleted', () =>
        problemResponse({
          status: 404,
          code: 'not_found',
          title: '资源不存在',
          detail: '课时不存在或已被删除。',
        }),
      ),
    )
    renderWithRouter('/dashboard')

    // 校验完成前记录短暂可见；404 确认后隐藏并清空本地记录
    await waitFor(() =>
      expect(screen.queryByRole('link', { name: /已删除的课时/ })).not.toBeInTheDocument(),
    )
    expect(await screen.findByText('还没有学习记录')).toBeInTheDocument()
    await waitFor(() => expect(loadRecentLearning()).toEqual([]))
  })

  it('打开课时工作区后写入学习记录', async () => {
    const { router } = renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}`)

    expect(await screen.findByRole('heading', { name: '随机变量与分布' })).toBeInTheDocument()
    expect(loadRecentLearning().map((entry) => entry.lessonId)).toContain(LESSON_RANDOM_ID)

    // 回到学习总览：刚访问的课时出现在「继续学习」
    await router.navigate({ to: '/dashboard' })
    expect(await screen.findByRole('link', { name: /随机变量与分布/ })).toBeInTheDocument()
  })
})
