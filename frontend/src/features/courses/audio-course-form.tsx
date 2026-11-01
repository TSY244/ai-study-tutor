import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useRef, useState, type FormEvent } from 'react'

import { createLesson } from '@/api/generated/endpoints/lessons/lessons'
import type { Course, Lesson } from '@/api/generated/model'
import { invalidateLessonCreated, invalidateMaterialImportAccepted } from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import { uploadMaterialWithProgress, type UploadProgress } from '@/api/upload'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { courseFormSchema, toCourseCreate } from '@/features/courses/course-form-schema'
import { useCreateCourse } from '@/features/courses/use-create-course'
import { createLessonIntentScope } from '@/features/lessons/use-lesson-mutations'
import {
  AUDIO_ACCEPT,
  AUDIO_FORMATS,
  AUDIO_LANGUAGES,
  MAX_AUDIO_UPLOAD_BYTES,
  validateAudioFile,
} from '@/features/materials/audio-file'
import { ImportErrorAlert } from '@/features/materials/import-error-alert'
import { importMaterialIntentScope } from '@/features/materials/use-import-material'
import { formatBytes } from '@/lib/upload-limits'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

async function runIntent<T>(scope: string, request: (key: string) => Promise<T>): Promise<T> {
  try {
    const result = await request(beginIdempotencyIntent(scope))
    resolveIdempotencyIntent(scope)
    return result
  } catch (error) {
    if (
      isApiProblemError(error) &&
      (error.code === 'validation_error' || error.code === 'idempotency_conflict')
    )
      rotateIdempotencyIntent(scope)
    throw error
  }
}

/** 上传失败时复用已经创建的课程和课时，重试继续同一次导入。 */
export function AudioCourseForm() {
  const client = useQueryClient()
  const navigate = useNavigate()
  const createCourse = useCreateCourse()
  const [course, setCourse] = useState<Course | null>(null)
  const [lesson, setLesson] = useState<Lesson | null>(null)
  const [title, setTitle] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [language, setLanguage] = useState('')
  const [validation, setValidation] = useState<string | null>(null)
  const [progress, setProgress] = useState<UploadProgress | null>(null)
  const [stage, setStage] = useState('')
  const inFlight = useRef(false)

  const mutation = useMutation({
    mutationFn: async (audio: File) => {
      setStage('创建课程…')
      const targetCourse =
        course ?? (await createCourse.mutateAsync(toCourseCreate({ title, description: '' })))
      setCourse(targetCourse)
      setStage('创建课时…')
      const targetLesson =
        lesson ??
        (await runIntent(createLessonIntentScope(targetCourse.id), (key) =>
          createLesson(
            targetCourse.id,
            {
              ordinal: 1,
              title: audio.name.replace(/\.[^.]+$/, '').slice(0, 200) || '音频课时',
            },
            { headers: { 'Idempotency-Key': key } },
          ),
        ))
      setLesson(targetLesson)
      await invalidateLessonCreated(client, targetCourse.id)
      setStage('上传音频…')
      const accepted = await runIntent(importMaterialIntentScope(targetLesson.id), (key) =>
        uploadMaterialWithProgress(
          targetLesson.id,
          {
            file: audio,
            transcription_mode: 'whisper_local',
            ...(language ? { preferred_language: language } : {}),
          },
          { idempotencyKey: key, onProgress: setProgress },
        ),
      )
      await invalidateMaterialImportAccepted(
        client,
        targetLesson.id,
        targetCourse.id,
        accepted.job.id,
      )
      await navigate({
        to: '/courses/$courseId/lessons/$lessonId/notes',
        params: { courseId: targetCourse.id, lessonId: targetLesson.id },
      })
    },
  })
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (inFlight.current) return
    const parsed = courseFormSchema.safeParse({ title, description: '' })
    const error = !parsed.success
      ? parsed.error.issues[0]?.message
      : !file
        ? '请选择音频文件。'
        : validateAudioFile(file)
    setValidation(error ?? null)
    if (error || !file) return
    inFlight.current = true
    setProgress(null)
    try {
      await mutation.mutateAsync(file)
    } catch {
      /* Mutation error remains visible with all inputs preserved. */
    } finally {
      inFlight.current = false
    }
  }

  return (
    <form noValidate onSubmit={(event) => void submit(event)} className="max-w-xl space-y-5">
      <p className="text-sm text-text-muted">
        上传课堂录音、讲座或播客，先转成文本，再由 AI 按课程统一格式总结；可继续用知识点生成测验。
      </p>
      {mutation.isError ? <ImportErrorAlert error={mutation.error} /> : null}
      {validation ? (
        <p role="alert" className="text-sm text-danger">
          {validation}
        </p>
      ) : null}
      <div className="space-y-1.5">
        <Label htmlFor="audio-course-title">课程标题（可选）</Label>
        <Input
          id="audio-course-title"
          value={title}
          maxLength={200}
          disabled={mutation.isPending || course !== null}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="留空，由 AI 根据录音内容命名"
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="audio-course-file" required>
          音频文件
        </Label>
        <input
          id="audio-course-file"
          type="file"
          accept={AUDIO_ACCEPT}
          required
          disabled={mutation.isPending}
          aria-describedby="audio-course-hint"
          onChange={(event) => {
            setFile(event.target.files?.[0] ?? null)
            setValidation(null)
          }}
          className="block w-full text-sm text-text file:mr-3 file:rounded-md file:border file:border-border-strong file:bg-surface file:px-3 file:py-1.5 file:text-sm file:text-text"
        />
        <p id="audio-course-hint" className="text-xs text-text-subtle">
          支持 {AUDIO_FORMATS}；最大 {formatBytes(MAX_AUDIO_UPLOAD_BYTES)}。
        </p>
        {file ? (
          <p className="text-sm text-text-muted">
            已选择：{file.name}（{formatBytes(file.size)}）
          </p>
        ) : null}
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="audio-course-language">音频语言</Label>
        <select
          id="audio-course-language"
          value={language}
          disabled={mutation.isPending}
          onChange={(event) => setLanguage(event.target.value)}
          className="h-9 w-full rounded-md border border-border-strong bg-surface px-3 text-sm text-text"
        >
          {AUDIO_LANGUAGES.map((item) => (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          ))}
        </select>
      </div>
      {mutation.isPending ? (
        <div role="status" aria-live="polite" className="space-y-2 text-sm text-text-muted">
          <p>
            {stage}
            {progress?.percent !== undefined ? ` ${progress.percent}%` : ''}
          </p>
          <p>上传完成后会显示后台转写与 AI 生成进度，原文会保留在课时中。</p>
        </div>
      ) : null}
      {course && mutation.isError ? (
        <p className="text-sm text-text-muted">
          课程已创建，重试会继续上传，不会重复创建。
          <Link
            to="/courses/$courseId"
            params={{ courseId: course.id }}
            className="ml-2 text-accent underline"
          >
            查看已创建的课程
          </Link>
        </p>
      ) : null}
      <Button type="submit" disabled={mutation.isPending}>
        {mutation.isPending ? stage : course ? '重试音频导入' : '创建并导入音频'}
      </Button>
    </form>
  )
}
