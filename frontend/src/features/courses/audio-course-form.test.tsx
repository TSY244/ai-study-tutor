import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { MaterialJobAccepted } from '@/api/generated/model'
import { uploadMaterialWithProgress } from '@/api/upload'
import { ApiProblemError } from '@/api/problem'
import { demoJobs, COURSE_STATS_ID, LESSON_CONFIDENCE_ID } from '@/mocks/fixtures'
import { captureRequests } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

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
      original_name: '讲座.mp3',
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

async function chooseAudio(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('tab', { name: '音频导入' }))
  const file = new File(['ID3音频内容'], '讲座.mp3', { type: 'audio/mpeg' })
  await user.upload(screen.getByLabelText(/音频文件/, { selector: 'input' }), file)
  return file
}

describe('音频创建课程', () => {
  it('按课程、课时、音频上传顺序执行，标题可留空并自动启用转写', async () => {
    const user = userEvent.setup()
    const courses = captureRequests('POST', '/api/v1/courses')
    const lessons = captureRequests('POST', /\/api\/v1\/courses\/[^/]+\/lessons$/)
    upload.mockImplementation(async (id) => {
      expect(courses.requests).toHaveLength(1)
      expect(lessons.requests).toHaveLength(1)
      return accepted(id)
    })
    const { router } = renderWithRouter('/courses/new')
    const file = await chooseAudio(user)
    await user.selectOptions(screen.getByLabelText('音频语言'), 'zh')
    await user.click(screen.getByRole('button', { name: '创建并导入音频' }))
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1))
    const [id, body] = upload.mock.calls[0]!
    expect(body).toEqual({ file, preferred_language: 'zh', transcription_mode: 'whisper_local' })
    expect(courses.requests[0]?.body).toEqual({})
    expect(lessons.requests[0]?.body).toEqual({ ordinal: 1, title: '讲座' })
    await waitFor(() => expect(router.state.location.pathname).toContain(`/lessons/${id}/notes`))
    courses.stop()
    lessons.stop()
  })

  it('上传网络失败后保留输入，重试复用课程、课时和幂等键', async () => {
    const user = userEvent.setup()
    const courses = captureRequests('POST', '/api/v1/courses')
    const lessons = captureRequests('POST', /\/api\/v1\/courses\/[^/]+\/lessons$/)
    upload.mockRejectedValueOnce(
      new ApiProblemError({
        status: 0,
        code: 'network_error',
        title: '网络错误',
        detail: '',
        requestId: 'test',
      }),
    )
    upload.mockImplementationOnce(async (id) => accepted(id))
    renderWithRouter('/courses/new')
    await chooseAudio(user)
    await user.type(screen.getByLabelText('课程标题（可选）'), '我的录音课')
    await user.click(screen.getByRole('button', { name: '创建并导入音频' }))
    expect(await screen.findByText(/课程已创建，重试会继续上传/)).toBeInTheDocument()
    expect(screen.getByLabelText('课程标题（可选）')).toHaveValue('我的录音课')
    expect(screen.getByText(/已选择：讲座.mp3/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '重试音频导入' }))
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(2))
    expect(courses.requests).toHaveLength(1)
    expect(lessons.requests).toHaveLength(1)
    expect(upload.mock.calls[0]?.[0]).toBe(upload.mock.calls[1]?.[0])
    expect(upload.mock.calls[0]?.[2]?.idempotencyKey).toBe(
      upload.mock.calls[1]?.[2]?.idempotencyKey,
    )
    courses.stop()
    lessons.stop()
  })

  it('无文件和超过 2GB 时不创建课程', async () => {
    const user = userEvent.setup()
    const requests = captureRequests('POST', '/api/v1/courses')
    renderWithRouter('/courses/new')
    await user.click(await screen.findByRole('tab', { name: '音频导入' }))
    await user.click(screen.getByRole('button', { name: '创建并导入音频' }))
    expect(screen.getByRole('alert')).toHaveTextContent('请选择音频文件')
    const file = new File(['x'], 'large.wav', { type: 'audio/wav' })
    Object.defineProperty(file, 'size', { value: 2 * 1024 ** 3 + 1 })
    await user.upload(screen.getByLabelText(/音频文件/, { selector: 'input' }), file)
    await user.click(screen.getByRole('button', { name: '创建并导入音频' }))
    expect(screen.getByRole('alert')).toHaveTextContent('超过 2.00 GB')
    expect(requests.requests).toHaveLength(0)
    requests.stop()
  })

  it('已有课时的音频入口使用相同上传 API，默认转写并保留语言选择', async () => {
    const user = userEvent.setup()
    upload.mockImplementation(async (id) => accepted(id))
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_CONFIDENCE_ID}/notes`)
    await user.click(await screen.findByRole('tab', { name: '音频' }))
    const file = new File(['fLaC123'], '录音.flac', { type: 'audio/flac' })
    Object.defineProperty(file, 'size', { value: 101 * 1024 * 1024 })
    await user.upload(screen.getByLabelText(/音频文件/, { selector: 'input' }), file)
    await user.selectOptions(screen.getByLabelText('音频语言'), 'en')
    await user.click(screen.getByRole('button', { name: '上传音频并生成内容' }))
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1))
    expect(upload.mock.calls[0]?.[0]).toBe(LESSON_CONFIDENCE_ID)
    expect(upload.mock.calls[0]?.[1]).toEqual({
      file,
      transcription_mode: 'whisper_local',
      preferred_language: 'en',
    })
  })
})
