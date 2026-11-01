import { useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useRef, useState, type FormEvent } from 'react'

import { createLesson, listLessons } from '@/api/generated/endpoints/lessons/lessons'
import type { Lesson, LessonCreate } from '@/api/generated/model'
import { invalidateLessonCreated, invalidateMaterialImportAccepted } from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { uploadMaterialWithProgress } from '@/api/upload'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { courseFormSchema, toCourseCreate } from '@/features/courses/course-form-schema'
import { useCreateCourse } from '@/features/courses/use-create-course'
import { isAudioFile, MAX_AUDIO_UPLOAD_BYTES } from '@/features/materials/audio-file'
import { ImportErrorAlert } from '@/features/materials/import-error-alert'
import { formatBytes, MAX_UPLOAD_BYTES } from '@/lib/upload-limits'

export type FileImportMode = 'images' | 'pptx' | 'batch'
const FORMATS = {
  images: '.png,.jpg,.jpeg,.webp,.tif,.tiff,.bmp,.gif,.pdf',
  pptx: '.pptx',
  batch:
    '.png,.jpg,.jpeg,.webp,.tif,.tiff,.bmp,.gif,.pdf,.pptx,.docx,.txt,.md,.markdown,.srt,.vtt,.html,.htm,.mp3,.wav,.m4a,.flac,.ogg,.opus,.aac,.mp4,.mkv,.webm,.mov',
}
const GUIDANCE = {
  images: '上传图片或扫描 PDF，先识别中英文文字，再由 AI 总结。PDF 保留页码，扫描页自动进行 OCR。',
  pptx: '按幻灯片顺序提取文字、表格、演讲者备注、图表数据与图片文字，再由 AI 总结。',
  batch:
    '可混合选择图片、PDF、PPTX、Word、文本、字幕和音视频。每个文件建立一个课时，分别生成内容。',
}
interface ImportItem {
  id: string
  file: File
  createKey: string
  uploadKey: string
  input?: LessonCreate
  lesson?: Lesson
  jobId?: string
  status: string
  error?: string
}

async function nextOrdinal(courseId: string) {
  let ordinal = 1
  let cursor: string | undefined
  do {
    const page = await listLessons(courseId, { limit: 100, cursor })
    for (const lesson of page.items) ordinal = Math.max(ordinal, lesson.ordinal + 1)
    cursor = page.page_info.has_more ? (page.page_info.next_cursor ?? undefined) : undefined
  } while (cursor)
  return ordinal
}

/** One file per lesson; accepted items are never submitted again on retry. */
export function FileCourseForm({
  mode = 'batch',
  courseId,
}: {
  mode?: FileImportMode
  courseId?: string
}) {
  const client = useQueryClient()
  const createCourse = useCreateCourse()
  const targetId = useRef<string | undefined>(courseId)
  const inFlight = useRef(false)
  const [title, setTitle] = useState('')
  const [items, setItems] = useState<ImportItem[]>([])
  const [pending, setPending] = useState(false)
  const [started, setStarted] = useState(false)
  const [validation, setValidation] = useState<string | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [createdId, setCreatedId] = useState(courseId)
  const update = (id: string, patch: Partial<ImportItem>) =>
    setItems((previous) => previous.map((item) => (item.id === id ? { ...item, ...patch } : item)))

  const select = (files: File[]) => {
    setError(null)
    setValidation(null)
    if (files.length > 50) {
      setValidation('每批最多 50 个文件，请分批导入。')
      setItems([])
      return
    }
    const extensions = FORMATS[mode].split(',')
    for (const file of files) {
      if (!extensions.some((ext) => file.name.toLowerCase().endsWith(ext))) {
        setValidation(`不支持文件「${file.name}」，请选择此入口支持的格式。`)
        setItems([])
        return
      }
      const limit = isAudioFile(file) ? MAX_AUDIO_UPLOAD_BYTES : MAX_UPLOAD_BYTES
      if (file.size === 0 || file.size > limit) {
        setValidation(`文件「${file.name}」为空或超过 ${formatBytes(limit)} 限制。`)
        setItems([])
        return
      }
    }
    setItems(
      files.map((file) => ({
        id: crypto.randomUUID(),
        file,
        createKey: crypto.randomUUID(),
        uploadKey: crypto.randomUUID(),
        status: '待导入',
      })),
    )
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (inFlight.current) return
    const parsed = courseFormSchema.safeParse({ title, description: '' })
    if (!courseId && !parsed.success) {
      setValidation(parsed.error.issues[0]?.message ?? '标题无效。')
      return
    }
    if (!items.length) {
      setValidation('请选择要导入的文件。')
      return
    }
    inFlight.current = true
    setPending(true)
    setStarted(true)
    setError(null)
    setValidation(null)
    try {
      const id =
        targetId.current ??
        (await createCourse.mutateAsync(toCourseCreate({ title, description: '' }))).id
      targetId.current = id
      setCreatedId(id)
      let ordinal = await nextOrdinal(id)
      // Reserve the same ordinal/key after ambiguous errors, even if a previous
      // request committed. A new body must never be retried with the old key.
      for (const item of items) if (item.input) ordinal = Math.max(ordinal, item.input.ordinal + 1)
      for (const item of items) {
        if (item.jobId) continue
        update(item.id, { status: '创建课时…', error: undefined })
        try {
          const input = item.input ?? {
            ordinal: ordinal++,
            title: item.file.name.replace(/\.[^.]+$/, '').slice(0, 200) || '导入课时',
          }
          update(item.id, { input })
          const lesson =
            item.lesson ??
            (await createLesson(id, input, { headers: { 'Idempotency-Key': item.createKey } }))
          update(item.id, { lesson, status: '上传中…' })
          await invalidateLessonCreated(client, id)
          const accepted = await uploadMaterialWithProgress(
            lesson.id,
            {
              file: item.file,
              ...(isAudioFile(item.file) ? { transcription_mode: 'whisper_local' as const } : {}),
            },
            {
              idempotencyKey: item.uploadKey,
              onProgress: (progress) =>
                update(item.id, {
                  status:
                    progress.percent === undefined ? '上传中…' : `上传中 ${progress.percent}%`,
                }),
            },
          )
          update(item.id, { jobId: accepted.job.id, status: '已入队，后台生成中' })
          await invalidateMaterialImportAccepted(client, lesson.id, id, accepted.job.id)
        } catch (cause) {
          const problem = presentProblem(cause)
          update(item.id, {
            status: '导入失败',
            error: `${problem.title}：${problem.message}`,
            // Known rejected bodies may be edited/reallocated on the next try.
            ...(isApiProblemError(cause) &&
            (cause.code === 'validation_error' || cause.code === 'idempotency_conflict')
              ? {
                  createKey: crypto.randomUUID(),
                  uploadKey: crypto.randomUUID(),
                  ...(item.lesson ? {} : { input: undefined }),
                }
              : {}),
          })
        }
      }
    } catch (cause) {
      setError(cause)
    } finally {
      inFlight.current = false
      setPending(false)
    }
  }
  const remaining = items.filter((item) => !item.jobId).length
  return (
    <form noValidate onSubmit={(event) => void submit(event)} className="space-y-4">
      <p className="text-sm text-text-muted">
        {GUIDANCE[mode]} 每批最多 50 个文件，一个文件对应一个课时。
      </p>
      {error ? <ImportErrorAlert error={error} /> : null}
      {validation ? (
        <p role="alert" className="text-sm text-danger">
          {validation}
        </p>
      ) : null}
      {!courseId ? (
        <div className="space-y-1.5">
          <Label htmlFor={`file-course-title-${mode}`}>课程标题（可选）</Label>
          <Input
            id={`file-course-title-${mode}`}
            value={title}
            disabled={started}
            maxLength={200}
            onChange={(event) => setTitle(event.target.value)}
            placeholder="留空，由 AI 根据导入内容命名"
          />
        </div>
      ) : null}
      <div className="space-y-1.5">
        <Label htmlFor={`file-course-files-${mode}`} required>
          导入文件（可多选）
        </Label>
        <input
          id={`file-course-files-${mode}`}
          type="file"
          multiple
          accept={FORMATS[mode]}
          disabled={started}
          onChange={(event) => select(Array.from(event.target.files ?? []))}
          className="block w-full text-sm text-text file:mr-3 file:rounded-md file:border file:border-border-strong file:bg-surface file:px-3 file:py-1.5 file:text-sm file:text-text"
        />
        <p className="text-xs text-text-subtle">
          普通文件最大 {formatBytes(MAX_UPLOAD_BYTES)}，音频最大{' '}
          {formatBytes(MAX_AUDIO_UPLOAD_BYTES)}。PDF / PPTX 最多 300 页。
        </p>
      </div>
      {items.length ? (
        <ol
          aria-label="文件导入进度"
          aria-live="polite"
          className="divide-y divide-border rounded-md border border-border"
        >
          {items.map((item) => (
            <li key={item.id} className="space-y-1 p-3 text-sm">
              <div className="flex flex-wrap justify-between gap-2">
                <span className="break-all font-medium text-text">{item.file.name}</span>
                <span className="text-text-muted">{item.status}</span>
              </div>
              <p className="text-xs text-text-subtle">{formatBytes(item.file.size)}</p>
              {item.error ? (
                <p role="alert" className="text-danger">
                  {item.error}
                </p>
              ) : null}
              {item.lesson && createdId ? (
                <Link
                  to="/courses/$courseId/lessons/$lessonId/notes"
                  params={{ courseId: createdId, lessonId: item.lesson.id }}
                  className="text-accent underline"
                >
                  查看课时与生成进度
                </Link>
              ) : null}
              {!started ? (
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  aria-label={`移除 ${item.file.name}`}
                  onClick={() =>
                    setItems((previous) => previous.filter((row) => row.id !== item.id))
                  }
                >
                  移除
                </Button>
              ) : null}
            </li>
          ))}
        </ol>
      ) : null}
      <p className="text-xs text-text-muted">
        上传成功后可离开页面，后台会继续提取原文并由 AI 生成总结。上传失败可重试；AI
        生成失败请在课时或任务中心重试。
      </p>
      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" disabled={pending || (started && remaining === 0)}>
          {pending
            ? '正在导入…'
            : started
              ? remaining
                ? `重试未完成项（${remaining}）`
                : '全部已提交'
              : courseId
                ? '导入为新课时'
                : '创建课程并导入'}
        </Button>
        {createdId ? (
          <Link
            to="/courses/$courseId"
            params={{ courseId: createdId }}
            className="text-sm text-accent underline"
          >
            查看课程
          </Link>
        ) : null}
      </div>
    </form>
  )
}
