import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http } from 'msw'
import { describe, expect, it, vi } from 'vitest'

import type { MaterialJobAccepted } from '@/api/generated/model'
import { uploadMaterialWithProgress } from '@/api/upload'
import { isIdempotencyKey } from '@/lib/idempotency'
import {
  COURSE_STATS_ID,
  JOB_RUNNING_ID,
  LESSON_BAYES_ID,
  LESSON_CONFIDENCE_ID,
} from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { captureRequests, problemResponse } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

// jsdom 不支持 XHR 上传进度事件：组件测试 mock adapter（真实 XHR 由 E2E 覆盖）。
vi.mock('@/api/upload', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/upload')>()
  return { ...actual, uploadMaterialWithProgress: vi.fn() }
})

const FAILED_LESSON_URL = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_CONFIDENCE_ID}/notes`
const PROCESSING_LESSON_URL = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_BAYES_ID}/notes`
const TEXT_IMPORT_PATH = `/api/v1/lessons/${LESSON_CONFIDENCE_ID}/materials/text`

/**
 * 素材导入面板：三种来源、resource_busy 关联、上传与任务进度分离。
 * 演示课时：置信区间（failed，显示导入表单）、贝叶斯定理（processing，显示任务卡）。
 */
describe('ImportPanel', () => {
  it('failed 课时显示导入表单与失败提示', async () => {
    renderWithRouter(FAILED_LESSON_URL)
    expect(await screen.findByRole('heading', { name: '导入素材' })).toBeInTheDocument()
    expect(screen.getByText(/上次生成失败/)).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: '文本' })).toHaveAttribute('aria-selected', 'true')
  })

  it('processing 课时直接显示生成任务卡（不显示导入表单）', async () => {
    renderWithRouter(PROCESSING_LESSON_URL)
    expect(
      await screen.findByLabelText('生成任务进度', undefined, { timeout: 3000 }),
    ).toBeInTheDocument()
    expect(
      within(screen.getByRole('region', { name: '后台生成任务' })).getByRole('link', {
        name: '查看任务详情',
      }),
    ).toHaveAttribute('href', `/jobs/${JOB_RUNNING_ID}`)
    expect(screen.queryByRole('tab', { name: '文本' })).not.toBeInTheDocument()
  })

  it('文本导入：202 后精确失效并切换为任务进度（上传/任务进度分离）', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', TEXT_IMPORT_PATH)
    renderWithRouter(FAILED_LESSON_URL)

    await user.type(await screen.findByLabelText(/文本内容/), '置信区间的定义与解释。')
    await user.click(screen.getByRole('button', { name: '导入文本' }))

    // 202 接受：课时转为 processing，面板切换为后台任务进度
    expect(
      await screen.findByLabelText('生成任务进度', undefined, { timeout: 3000 }),
    ).toBeInTheDocument()
    expect(
      within(screen.getByRole('region', { name: '后台生成任务' })).getByRole('link', {
        name: '查看任务详情',
      }),
    ).toBeInTheDocument()

    // 请求带幂等 Key；任务中心（活动观察者）刷新后计数 +1（演示数据已有 1 个进行中）
    expect(requests).toHaveLength(1)
    expect(isIdempotencyKey(requests[0]?.key ?? '')).toBe(true)
    expect(requests[0]?.body).toEqual({ text: '置信区间的定义与解释。' })
    const trigger = screen.getByRole('button', { name: /任务中心/ })
    await waitFor(() => expect(trigger).toHaveAccessibleName(/2 个任务进行中/))
    stop()
  })

  it('resource_busy：关联现有任务链接，不换 Key、不建新任务', async () => {
    const user = userEvent.setup()
    server.use(
      http.post(`*/api/v1/lessons/:lessonId/materials/text`, () =>
        problemResponse({
          status: 409,
          code: 'resource_busy',
          detail: '同一课时同时只允许一个生成任务。',
          active_job_id: JOB_RUNNING_ID,
          generation_family: 'lesson_generation',
        } as never),
      ),
    )
    const { requests, stop } = captureRequests('POST', TEXT_IMPORT_PATH)
    renderWithRouter(FAILED_LESSON_URL)

    await user.type(await screen.findByLabelText(/文本内容/), '第一段文本。')
    await user.click(screen.getByRole('button', { name: '导入文本' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('已有任务在进行')
    const link = screen.getByRole('link', { name: '查看进行中的生成任务' })
    expect(link).toHaveAttribute('href', `/jobs/${JOB_RUNNING_ID}`)

    // 同一意图再次提交：复用同一 Key（服务端 409 不落库，重试安全）
    await user.click(screen.getByRole('button', { name: '导入文本' }))
    await waitFor(() => expect(requests).toHaveLength(2))
    expect(requests[0]?.key).toBe(requests[1]?.key)
    stop()
  })

  it('URL 导入：非 https 链接客户端拦截；合法链接提交成功', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests(
      'POST',
      `/api/v1/lessons/${LESSON_CONFIDENCE_ID}/materials/url`,
    )
    renderWithRouter(FAILED_LESSON_URL)

    await user.click(await screen.findByRole('tab', { name: '链接' }))
    // 说明文案：由 Claude 在后台提取允许的 https 链接
    expect(screen.getByText(/由 Claude 在后台提取/)).toBeInTheDocument()
    const urlInput = await screen.findByRole('textbox', { name: /链接/ })
    await user.type(urlInput, 'http://insecure.example.com/a')
    await user.click(screen.getByRole('button', { name: '导入链接' }))
    expect(await screen.findByText('只支持 https:// 开头的链接。')).toBeInTheDocument()
    expect(requests).toHaveLength(0)

    await user.clear(urlInput)
    await user.type(urlInput, 'https://example.com/stats')
    await user.click(screen.getByRole('button', { name: '导入链接' }))
    expect(
      await screen.findByLabelText('生成任务进度', undefined, { timeout: 3000 }),
    ).toBeInTheDocument()
    expect(requests).toHaveLength(1)
    expect(requests[0]?.body).toEqual({ url: 'https://example.com/stats' })
    stop()
  })

  it('文件上传：超限文件选择时即提示；合法文件走 XHR 通道（独立上传进度 + 幂等键）', async () => {
    const user = userEvent.setup()
    const mockUpload = vi.mocked(uploadMaterialWithProgress)
    mockUpload.mockReset()
    renderWithRouter(FAILED_LESSON_URL)
    await user.click(await screen.findByRole('tab', { name: '文件' }))

    const fileInput = await screen.findByLabelText(/文件/, { selector: 'input' })
    // 超限：客户端预检（101MB）直接提示，不发请求
    const oversized = new File(['x'], 'big.pdf', { type: 'application/pdf' })
    Object.defineProperty(oversized, 'size', { value: 101 * 1024 * 1024 })
    await user.upload(fileInput, oversized)
    expect(await screen.findByText(/超过限制/)).toBeInTheDocument()
    expect(mockUpload).not.toHaveBeenCalled()

    // 合法文件：XHR 通道上传（jsdom 不支持 XHR 进度事件，mock adapter；
    // 真实 XHR + MSW 端到端由 Playwright 覆盖）
    let settle: ((value: MaterialJobAccepted) => void) | undefined
    mockUpload.mockImplementation((_lessonId, _body, options) => {
      options?.onProgress?.({ loaded: 50, total: 100, percent: 50 })
      return new Promise((resolve) => {
        settle = resolve
      })
    })
    const valid = new File(['# 章节内容'], 'chapter.md', { type: 'text/markdown' })
    await user.upload(fileInput, valid)
    expect(screen.getByText(/chapter\.md/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '上传并导入' }))

    // 上传进度（独立于后台任务进度）展示中
    expect(await screen.findByLabelText('文件上传进度')).toBeInTheDocument()
    expect(screen.getByText('50%')).toBeInTheDocument()
    expect(mockUpload).toHaveBeenCalledTimes(1)
    const [lessonIdArg, bodyArg, optionsArg] = mockUpload.mock.calls[0] ?? []
    expect(lessonIdArg).toBe(LESSON_CONFIDENCE_ID)
    expect(bodyArg?.file).toBe(valid)
    expect(isIdempotencyKey(optionsArg?.idempotencyKey ?? '')).toBe(true)

    // 202 接受：上传态结束、无错误（面板切换为任务卡由 E2E 覆盖）
    const now = new Date().toISOString()
    settle?.({
      material: {
        id: 'f0000000-0000-4000-8000-000000000001',
        lesson_id: LESSON_CONFIDENCE_ID,
        kind: 'upload',
        status: 'pending',
        source_url: null,
        original_name: 'chapter.md',
        created_at: now,
      },
      job: {
        id: 'f0000000-0000-4000-8000-000000000002',
        kind: 'lesson_import',
        aggregate_type: 'lesson',
        aggregate_id: LESSON_CONFIDENCE_ID,
        generation_family: 'lesson_generation',
        retry_of_job_id: null,
        status: 'running',
        stage: '解析素材',
        progress: 10,
        message: null,
        attempt_count: 1,
        max_attempts: 3,
        retryable: false,
        cancel_requested: false,
        error: null,
        created_at: now,
        started_at: now,
        completed_at: null,
        updated_at: now,
      },
    })
    await waitFor(() => expect(screen.queryByLabelText('文件上传进度')).not.toBeInTheDocument())
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
