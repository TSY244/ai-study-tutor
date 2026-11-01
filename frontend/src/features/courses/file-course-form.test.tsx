import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse, http } from 'msw'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { MaterialJobAccepted } from '@/api/generated/model'
import { ApiProblemError } from '@/api/problem'
import { uploadMaterialWithProgress } from '@/api/upload'
import { FileCourseForm } from '@/features/courses/file-course-form'
import { COURSE_STATS_ID, demoJobs, demoLessons } from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { captureRequests } from '@/test/mock-helpers'
import { renderWithClient, renderWithRouter } from '@/test/utils'

vi.mock('@/api/upload', async (original) => ({
  ...(await original<typeof import('@/api/upload')>()),
  uploadMaterialWithProgress: vi.fn(),
}))
const upload = vi.mocked(uploadMaterialWithProgress)
function accepted(lessonId: string): MaterialJobAccepted {
  return {
    material: {
      id: crypto.randomUUID(),
      lesson_id: lessonId,
      kind: 'upload',
      status: 'pending',
      original_name: 'file',
      created_at: new Date().toISOString(),
    },
    job: {
      ...demoJobs[0]!,
      id: crypto.randomUUID(),
      kind: 'lesson_import',
      aggregate_type: 'lesson',
      aggregate_id: lessonId,
      generation_family: 'lesson_generation',
      status: 'queued',
    },
  }
}
beforeEach(() => {
  upload.mockReset()
})
async function selectFiles(user: ReturnType<typeof userEvent.setup>, names: string[]) {
  const files = names.map((name) => new File(['test content'], name))
  await user.upload(screen.getByLabelText(/导入文件/, { selector: 'input' }), files)
  return files
}

describe('课程文件导入', () => {
  it.each(['图片 / 扫描 PDF', 'PPTX 课件'])('新建课程有 %s 入口，标题可不填写', async (tab) => {
    const user = userEvent.setup()
    upload.mockImplementation(async (id) => accepted(id))
    const courses = captureRequests('POST', '/api/v1/courses')
    renderWithRouter('/courses/new')
    await user.click(await screen.findByRole('tab', { name: tab }))
    await selectFiles(user, [tab === 'PPTX 课件' ? '课件.pptx' : '扫描.pdf'])
    await user.click(screen.getByRole('button', { name: '创建课程并导入' }))
    await screen.findByRole('button', { name: '全部已提交' })
    expect(courses.requests[0]?.body).toEqual({})
    expect(upload).toHaveBeenCalledTimes(1)
    courses.stop()
  })

  it('混合批量导入部分失败后继续其他文件，重试不重复创建或提交成功项', async () => {
    const user = userEvent.setup()
    const courses = captureRequests('POST', '/api/v1/courses')
    const lessons = captureRequests('POST', /\/api\/v1\/courses\/[^/]+\/lessons$/)
    upload.mockImplementation(async (id, body) => {
      if ((body.file as File).name === '扫描.pdf' && upload.mock.calls.length === 2)
        throw new ApiProblemError({
          status: 0,
          code: 'network_error',
          title: '网络错误',
          detail: '',
          requestId: 'test',
        })
      return accepted(id)
    })
    renderWithRouter('/courses/new')
    await user.click(await screen.findByRole('tab', { name: '批量文件' }))
    const files = await selectFiles(user, ['图片.png', '扫描.pdf', '课件.pptx'])
    await user.click(screen.getByRole('button', { name: '创建课程并导入' }))
    await screen.findByRole('button', { name: '重试未完成项（1）' })
    expect(upload).toHaveBeenCalledTimes(3)
    expect(screen.getAllByText('已入队，后台生成中')).toHaveLength(2)
    const failedCall = upload.mock.calls[1]!
    await user.click(screen.getByRole('button', { name: '重试未完成项（1）' }))
    await screen.findByRole('button', { name: '全部已提交' })
    expect(upload).toHaveBeenCalledTimes(4)
    expect(courses.requests).toHaveLength(1)
    expect(lessons.requests).toHaveLength(3)
    expect(
      lessons.requests.map((request) => (request.body as { ordinal: number }).ordinal),
    ).toEqual([1, 2, 3])
    expect(upload.mock.calls[3]?.[0]).toBe(failedCall[0])
    expect(upload.mock.calls[3]?.[2].idempotencyKey).toBe(failedCall[2].idempotencyKey)
    expect(upload.mock.calls[3]?.[1].file).toBe(files[1])
    courses.stop()
    lessons.stop()
  })

  it('创建请求已落库但响应丢失时，重试保留原序号和幂等键', async () => {
    const user = userEvent.setup()
    const created = new Map<number, (typeof demoLessons)[number]>()
    let loseResponse = true
    server.use(
      http.get('*/api/v1/courses/:courseId/lessons', () =>
        HttpResponse.json({
          items: [...created.values()],
          page_info: { has_more: false, next_cursor: null },
        }),
      ),
      http.post('*/api/v1/courses/:courseId/lessons', async ({ request, params }) => {
        const body = (await request.json()) as { ordinal: number; title: string }
        const lesson = created.get(body.ordinal) ?? {
          ...demoLessons[0]!,
          id: crypto.randomUUID(),
          course_id: String(params.courseId),
          ordinal: body.ordinal,
          title: body.title,
        }
        created.set(body.ordinal, lesson)
        if (loseResponse) {
          loseResponse = false
          return HttpResponse.error()
        }
        return HttpResponse.json(lesson, { status: 201 })
      }),
    )
    const lessons = captureRequests('POST', /\/api\/v1\/courses\/[^/]+\/lessons$/)
    upload.mockImplementation(async (id) => accepted(id))
    renderWithRouter('/courses/new')
    await user.click(await screen.findByRole('tab', { name: '批量文件' }))
    await selectFiles(user, ['第一讲.pdf', '第二讲.pdf'])
    await user.click(screen.getByRole('button', { name: '创建课程并导入' }))
    await user.click(await screen.findByRole('button', { name: '重试未完成项（1）' }))
    await screen.findByRole('button', { name: '全部已提交' })
    expect(created.size).toBe(2)
    expect(upload).toHaveBeenCalledTimes(2)
    expect(lessons.requests).toHaveLength(3)
    expect(lessons.requests[0]?.body).toEqual(lessons.requests[2]?.body)
    expect(lessons.requests[0]?.key).toBe(lessons.requests[2]?.key)
    lessons.stop()
  })

  it('已有课程批量入口会读取所有分页，追加到最大课时序号后', async () => {
    const user = userEvent.setup()
    const template = demoLessons.find((lesson) => lesson.course_id === COURSE_STATS_ID)!
    server.use(
      http.get('*/api/v1/courses/:courseId/lessons', ({ request }) => {
        const cursor = new URL(request.url).searchParams.get('cursor')
        return HttpResponse.json({
          items: [{ ...template, ordinal: cursor ? 80 : 10 }],
          page_info: { has_more: !cursor, next_cursor: cursor ? null : 'page2' },
        })
      }),
    )
    const courses = captureRequests('POST', '/api/v1/courses')
    const lessons = captureRequests('POST', /\/api\/v1\/courses\/[^/]+\/lessons$/)
    upload.mockImplementation(async (id) => accepted(id))
    renderWithRouter(`/courses/${COURSE_STATS_ID}`)
    await user.click(await screen.findByRole('button', { name: '批量导入' }))
    await selectFiles(user, ['补充资料.pptx'])
    await user.click(screen.getByRole('button', { name: '导入为新课时' }))
    await screen.findByRole('button', { name: '全部已提交' })
    expect(courses.requests).toHaveLength(0)
    expect(lessons.requests[0]?.body).toMatchObject({ ordinal: 81, title: '补充资料' })
    courses.stop()
    lessons.stop()
  })

  it('无文件、超数量、超体积、错误格式都在创建前拒绝', async () => {
    const user = userEvent.setup({ applyAccept: false })
    renderWithClient(<FileCourseForm />)
    const button = screen.getByRole('button', { name: '创建课程并导入' })
    const input = screen.getByLabelText(/导入文件/, { selector: 'input' })
    await user.click(button)
    expect(screen.getByRole('alert')).toHaveTextContent('请选择')
    await user.upload(
      input,
      Array.from({ length: 51 }, (_, i) => new File(['x'], `${i}.pdf`)),
    )
    expect(screen.getByRole('alert')).toHaveTextContent('最多 50')
    const oversized = new File(['x'], '大图.png')
    Object.defineProperty(oversized, 'size', { value: 101 * 1024 * 1024 })
    await user.upload(input, oversized)
    expect(screen.getByRole('alert')).toHaveTextContent('超过')
    await user.upload(input, new File(['x'], '不支持.exe'))
    expect(screen.getByRole('alert')).toHaveTextContent('不支持')
    expect(upload).not.toHaveBeenCalled()
  })

  it('批量音频自动启用本地转写', async () => {
    const user = userEvent.setup()
    upload.mockImplementation(async (id) => accepted(id))
    renderWithRouter('/courses/new')
    await user.click(await screen.findByRole('tab', { name: '批量文件' }))
    await selectFiles(user, ['录音.flac'])
    await user.click(screen.getByRole('button', { name: '创建课程并导入' }))
    await screen.findByRole('button', { name: '全部已提交' })
    await waitFor(() =>
      expect(upload.mock.calls[0]?.[1]).toMatchObject({ transcription_mode: 'whisper_local' }),
    )
  })
})
