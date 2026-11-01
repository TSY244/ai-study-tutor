import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { delay, http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { queryKeys } from '@/api/query-client'
import { CREATE_COURSE_INTENT_SCOPE } from '@/features/courses/use-create-course'
import { isIdempotencyKey, peekIdempotencyIntent } from '@/lib/idempotency'
import { server } from '@/mocks/server'
import { problemResponse } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

/**
 * 新建课程表单：F2 验收的四条主线——
 * 表单错误绑定、重复提交保护、错误不清空输入、Idempotency-Key 生命周期。
 * 渲染走真实路由树 + MSW（curated handlers 模拟服务端幂等语义）。
 */

interface CapturedRequest {
  key: string | null
  body: unknown
}

/** 用 MSW 生命周期事件旁路记录 POST /courses（不替换 curated handler）。 */
function captureCoursePosts(): { requests: CapturedRequest[]; stop: () => void } {
  const requests: CapturedRequest[] = []
  const listener = async ({ request }: { request: Request }) => {
    if (request.method === 'POST' && new URL(request.url).pathname === '/api/v1/courses') {
      requests.push({
        key: request.headers.get('Idempotency-Key'),
        body: await request
          .clone()
          .json()
          .catch(() => null),
      })
    }
  }
  server.events.on('request:start', listener)
  return { requests, stop: () => server.events.removeListener('request:start', listener) }
}

function courseJson(title: string) {
  const now = new Date().toISOString()
  return {
    id: globalThis.crypto.randomUUID(),
    title,
    description: '',
    archived: false,
    lesson_count: 0,
    version: 1,
    created_at: now,
    updated_at: now,
  }
}

describe('NewCourseForm', () => {
  it('课程标题可留空，创建后等待 AI 命名', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureCoursePosts()
    const { router } = renderWithRouter('/courses/new')
    const titleInput = await screen.findByLabelText(/课程标题/)
    expect(titleInput).not.toBeRequired()
    await user.type(titleInput, '   ')
    await user.click(screen.getByRole('button', { name: '创建课程' }))
    await waitFor(() =>
      expect(router.state.location.pathname).toMatch(/^\/courses\/[0-9a-f-]{36}$/),
    )
    expect(await screen.findByRole('heading', { name: '待 AI 命名' })).toBeInTheDocument()
    expect(requests).toHaveLength(1)
    expect(requests[0]?.body).toEqual({})
    stop()
  })

  it('创建成功：带 Idempotency-Key，意图结束，课程列表缓存精确失效，进入课程详情', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureCoursePosts()
    const { client, router } = renderWithRouter('/courses/new')

    // 预置列表缓存，验证成功后按资源关系失效（而非全局失效）
    client.setQueryData(queryKeys.courses({ limit: 20 }), { items: [], page_info: {} })

    await user.type(await screen.findByLabelText(/课程标题/), '  线性代数  ')
    await user.type(screen.getByLabelText(/课程描述/), '矩阵与向量空间。')
    await user.click(screen.getByRole('button', { name: '创建课程' }))

    await waitFor(() =>
      expect(router.state.location.pathname).toMatch(/^\/courses\/[0-9a-f-]{36}$/),
    )
    // 详情页（curated MSW 可查到新课程）展示课程标题
    expect(await screen.findByRole('heading', { name: '线性代数' })).toBeInTheDocument()

    expect(requests).toHaveLength(1)
    expect(isIdempotencyKey(requests[0]?.key ?? '')).toBe(true)
    // 载荷去空白；空描述省略字段
    expect(requests[0]?.body).toEqual({ title: '线性代数', description: '矩阵与向量空间。' })

    // 意图已结束（下次创建是新 Key）；列表缓存被精确失效
    expect(peekIdempotencyIntent(CREATE_COURSE_INTENT_SCOPE)).toBeUndefined()
    expect(client.getQueryState(queryKeys.courses({ limit: 20 }))?.isInvalidated).toBe(true)
    stop()
  })

  it('重复提交保护：提交中按钮禁用，同步连点只产生一次请求', async () => {
    const user = userEvent.setup()
    let postCount = 0
    server.use(
      http.post('*/api/v1/courses', async () => {
        postCount += 1
        await delay(50)
        return HttpResponse.json(courseJson('概率论'), { status: 201 })
      }),
    )
    renderWithRouter('/courses/new')

    await user.type(await screen.findByLabelText(/课程标题/), '概率论')
    const submit = screen.getByRole('button', { name: '创建课程' })
    await user.click(submit)

    // 提交进行中：按钮禁用 + 文案切换（加载状态）
    await waitFor(() => expect(submit).toBeDisabled())
    expect(screen.getByRole('button', { name: '创建中…' })).toBeDisabled()

    // 双击的第二次/第三次到达时提交尚未完成，被防重守卫拦截
    fireEvent.click(submit)
    fireEvent.click(submit)

    await waitFor(() => expect(postCount).toBe(1))
    expect(postCount).toBe(1)
  })

  it('validation_error：字段错误绑定 + 未知字段入汇总 + 输入保留 + 修正后换新 Key', async () => {
    const user = userEvent.setup()
    const keys: (string | null)[] = []
    let shouldFail = true
    server.use(
      http.post('*/api/v1/courses', async ({ request }) => {
        keys.push(request.headers.get('Idempotency-Key'))
        if (shouldFail) {
          return problemResponse({
            status: 422,
            code: 'validation_error',
            detail: '字段校验失败。',
            errors: [
              { field: 'title', code: 'taken', message: '课程标题已存在。' },
              { field: 'legacy_field', code: 'unexpected', message: '未知字段不被接受。' },
            ],
          })
        }
        return HttpResponse.json(courseJson('统计学'), { status: 201 })
      }),
    )
    renderWithRouter('/courses/new')

    const titleInput = await screen.findByLabelText(/课程标题/)
    await user.type(titleInput, '统计学')
    await user.click(screen.getByRole('button', { name: '创建课程' }))

    // 已知字段绑定到输入下方
    expect(await screen.findByText('课程标题已存在。')).toBeInTheDocument()
    expect(titleInput).toHaveAttribute('aria-invalid', 'true')
    // 未知字段不丢失，进入顶部汇总；汇总带 code 与 X-Request-ID 诊断
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('表单未通过校验')
    expect(alert).toHaveTextContent('legacy_field：未知字段不被接受。')
    expect(alert).toHaveTextContent('validation_error')
    expect(alert).toHaveTextContent('req-test-1')
    await waitFor(() => expect(alert).toHaveFocus())
    // 错误不清空输入
    expect(titleInput).toHaveValue('统计学')

    // 修正后重试：载荷变化，按生命周期轮换为新 Key
    shouldFail = false
    await user.clear(titleInput)
    await user.type(titleInput, '统计学（重修）')
    await user.click(screen.getByRole('button', { name: '创建课程' }))

    await waitFor(() => expect(keys).toHaveLength(2))
    expect(keys[0]).not.toBe(keys[1])
    expect(isIdempotencyKey(keys[1] ?? '')).toBe(true)
  })

  it('network_error：输入保留，重试复用同一 Idempotency-Key', async () => {
    const user = userEvent.setup()
    const keys: (string | null)[] = []
    let calls = 0
    server.use(
      http.post('*/api/v1/courses', ({ request }) => {
        calls += 1
        keys.push(request.headers.get('Idempotency-Key'))
        if (calls === 1) return HttpResponse.error()
        return HttpResponse.json(courseJson('统计学'), { status: 201 })
      }),
    )
    const { router } = renderWithRouter('/courses/new')

    const titleInput = await screen.findByLabelText(/课程标题/)
    await user.type(titleInput, '统计学')
    await user.click(screen.getByRole('button', { name: '创建课程' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('网络连接失败')
    expect(alert).toHaveTextContent('network_error')
    // 错误不清空输入
    expect(titleInput).toHaveValue('统计学')

    // 同一意图重试：复用同一 Key（服务端据此去重）
    await user.click(screen.getByRole('button', { name: '创建课程' }))
    await waitFor(() => expect(router.state.location.pathname).toMatch(/^\/courses\//))
    expect(keys).toHaveLength(2)
    expect(keys[0]).toBe(keys[1])
  })

  it('MSW 幂等语义：同 Key 重放返回同一课程；同 Key 不同载荷判冲突；非法载荷 422', async () => {
    const key = '771f7e5f-1c92-4b8c-9ec4-a74ed6ef6bea'
    const url = 'http://localhost/api/v1/courses'
    const post = (body: unknown, idempotencyKey = key) =>
      fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey },
        body: JSON.stringify(body),
      })

    const first = await post({ title: '幂等课程' })
    const replay = await post({ title: '幂等课程' })
    expect(first.status).toBe(201)
    expect(replay.status).toBe(201)
    const created = (await first.json()) as { id: string }
    const replayed = (await replay.json()) as { id: string }
    expect(replayed.id).toBe(created.id)

    // 重放没有重复创建
    const list = (await (await fetch(url)).json()) as { items: { title: string }[] }
    expect(list.items.filter((item) => item.title === '幂等课程')).toHaveLength(1)

    // 同 Key 不同载荷 → idempotency_conflict
    const conflict = await post({ title: '另一门课' })
    expect(conflict.status).toBe(409)
    expect(((await conflict.json()) as { code: string }).code).toBe('idempotency_conflict')

    // 非法载荷 → 422 validation_error + 字段错误（契约 04 §3）
    const invalid = await post({ title: '长'.repeat(201) }, '771f7e5f-1c92-4b8c-9ec4-a74ed6ef6bec')
    expect(invalid.status).toBe(422)
    const problem = (await invalid.json()) as { code: string; errors: { field: string }[] }
    expect(problem.code).toBe('validation_error')
    expect(problem.errors[0]?.field).toBe('title')
  })
})
