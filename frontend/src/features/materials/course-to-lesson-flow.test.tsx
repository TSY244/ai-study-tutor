import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { renderWithRouter } from '@/test/utils'

/**
 * F3 验收主流程（MSW 集成）：创建课程 -> 新建课时 -> 导入文本素材 ->
 * 课时进入生成中 -> 离开页面后任务仍在全局任务中心与 /jobs 可见。
 */
describe('课程到课时导入全流程', () => {
  it('创建课程 -> 导入文本课时 -> 任务全局可见', async () => {
    const user = userEvent.setup()
    const { router } = renderWithRouter('/courses/new')

    // 1. 创建课程
    await user.type(await screen.findByLabelText(/课程标题/), '集成流程课')
    await user.click(screen.getByRole('button', { name: '创建课程' }))
    await waitFor(() =>
      expect(router.state.location.pathname).toMatch(/^\/courses\/[0-9a-f-]{36}$/),
    )
    const coursePath = router.state.location.pathname

    // 2. 新建课时（课次默认 1）——空课程页有两个"新建课时"入口（节区按钮 + 空状态），点第一个
    await user.click((await screen.findAllByRole('link', { name: '新建课时' }))[0]!)
    await waitFor(() => expect(router.state.location.pathname).toBe(`${coursePath}/lessons/new`))
    expect(await screen.findByLabelText(/课次/)).toHaveValue(1)
    await user.type(await screen.findByLabelText(/课时标题/), '第一课时')
    await user.click(screen.getByRole('button', { name: '创建课时' }))

    // 3. 课时页显示导入面板；导入文本
    await waitFor(() =>
      expect(router.state.location.pathname).toMatch(
        new RegExp(`^${coursePath}/lessons/[0-9a-f-]{36}`),
      ),
    )
    await user.type(await screen.findByLabelText(/文本内容/), '第一章：导数与极限。')
    await user.click(screen.getByRole('button', { name: '导入文本' }))

    // 4. 课时转入生成中：后台任务进度卡可见（与上传进度分离），记录新任务 id
    expect(
      await screen.findByLabelText('生成任务进度', undefined, { timeout: 3000 }),
    ).toBeInTheDocument()
    const jobDetailHref = within(screen.getByRole('region', { name: '后台生成任务' }))
      .getByRole('link', { name: '查看任务详情' })
      .getAttribute('href')
    expect(jobDetailHref).toMatch(/^\/jobs\/[0-9a-f-]{36}$/)

    // 5. 离开页面：任务中心角标与 /jobs 列表仍可见任务
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /任务中心/ })).toHaveAccessibleName(/任务进行中/),
    )

    // 离开课时页（主导航第一项为学习总览）
    await user.click(screen.getByRole('navigation', { name: '主导航' }).querySelector('a')!)
    await waitFor(() => expect(router.state.location.pathname).toBe('/dashboard'))
    // 任务中心在全局壳中仍可见；导航后重新查询 trigger（旧引用已失效）
    await user.click(screen.getByRole('button', { name: /任务中心/ }))
    await user.click(await screen.findByRole('link', { name: '查看全部' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/jobs'))
    // demo 数据中也有其他"课时导入"任务：按 href 断言新建任务在全局列表中
    const jobLinks = await screen.findAllByRole('link', { name: /课时导入/ })
    expect(jobLinks.some((el) => el.getAttribute('href') === jobDetailHref)).toBe(true)
  })
})
