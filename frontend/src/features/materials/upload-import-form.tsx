import {
  AUDIO_ACCEPT,
  AUDIO_FORMATS,
  AUDIO_LANGUAGES,
  MAX_AUDIO_UPLOAD_BYTES,
  isAudioFile,
  validateAudioFile,
} from '@/features/materials/audio-file'
import { useEffect, useRef, useState, type ChangeEvent } from 'react'

import type { Lesson } from '@/api/generated/model'
import type { UploadProgress } from '@/api/upload'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { ImportErrorAlert } from '@/features/materials/import-error-alert'
import { useImportMaterial } from '@/features/materials/use-import-material'
import { formatBytes, MAX_UPLOAD_BYTES } from '@/lib/upload-limits'

const TRANSCRIPTION_MODES = [
  { value: '', label: '自动（音频/视频才需要）' },
  { value: 'subtitle_only', label: '仅字幕' },
  { value: 'whisper_local', label: '本地 Whisper 转写' },
] as const

/**
 * 文件上传导入（02 §6.1：上传进度与后台 Job 进度分离）：
 * - 选择文件后立即展示名称/大小/限制；超限在选择时即提示（服务端 413 兜底）；
 * - 上传中显示独立“上传进度”（XHR progress）；
 * - 202 接受后上传完成 ≠ 导入完成：课时面板切换为后台“生成任务进度”。
 */
export function UploadImportForm({
  lesson,
  audioOnly = false,
}: {
  lesson: Lesson
  audioOnly?: boolean
}) {
  const { importUpload } = useImportMaterial(lesson)
  const alertRef = useRef<HTMLDivElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [fileError, setFileError] = useState<string | null>(null)
  const [transcriptionMode, setTranscriptionMode] = useState(audioOnly ? 'whisper_local' : '')
  const [preferredLanguage, setPreferredLanguage] = useState('')
  const [progress, setProgress] = useState<UploadProgress | null>(null)
  const [submitError, setSubmitError] = useState<unknown>(null)

  const pending = importUpload.isPending

  useEffect(() => {
    if (submitError) alertRef.current?.focus()
  }, [submitError])

  const onFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const selected = event.target.files?.[0] ?? null
    setSubmitError(null)
    setProgress(null)
    if (selected && audioOnly) {
      const error = validateAudioFile(selected)
      if (error) {
        setFile(null)
        setFileError(error)
        return
      }
    }
    const limit = selected && isAudioFile(selected) ? MAX_AUDIO_UPLOAD_BYTES : MAX_UPLOAD_BYTES
    if (selected && selected.size > limit) {
      setFile(null)
      setFileError(`文件 ${formatBytes(selected.size)} 超过限制 ${formatBytes(limit)}。`)
      return
    }
    setFileError(null)
    setFile(selected)
  }

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitError(null)
    if (!file) {
      setFileError('请选择要上传的文件。')
      fileInputRef.current?.focus()
      return
    }
    try {
      await importUpload.mutateAsync({
        body: {
          file,
          ...(preferredLanguage.trim() ? { preferred_language: preferredLanguage.trim() } : {}),
          ...(transcriptionMode || (isAudioFile(file) ? 'whisper_local' : '')
            ? {
                transcription_mode: (transcriptionMode || 'whisper_local') as
                  'subtitle_only' | 'whisper_local',
              }
            : {}),
        },
        onProgress: setProgress,
      })
    } catch (error) {
      setSubmitError(error)
      setProgress(null)
    }
  }

  return (
    <form noValidate onSubmit={(event) => void onSubmit(event)} className="space-y-4">
      {submitError ? <ImportErrorAlert ref={alertRef} error={submitError} /> : null}

      <div className="space-y-1.5">
        <Label htmlFor="upload-import-file" required>
          {audioOnly ? '音频文件' : '文件'}
        </Label>
        <input
          ref={fileInputRef}
          id="upload-import-file"
          type="file"
          accept={audioOnly ? AUDIO_ACCEPT : undefined}
          disabled={pending}
          required
          aria-invalid={fileError ? true : undefined}
          aria-describedby="upload-import-file-hint upload-import-file-error"
          onChange={onFileChange}
          className="block w-full text-sm text-text file:mr-3 file:rounded-md file:border file:border-border-strong file:bg-surface file:px-3 file:py-1.5 file:text-sm file:text-text hover:file:bg-surface-sunken"
        />
        <p id="upload-import-file-hint" className="text-xs text-text-subtle">
          {audioOnly
            ? `支持 ${AUDIO_FORMATS}；最大 ${formatBytes(MAX_AUDIO_UPLOAD_BYTES)}。`
            : `支持图片、扫描 PDF、PPTX、Word、Markdown、音频/视频；普通文件最大 ${formatBytes(MAX_UPLOAD_BYTES)}，音频最大 ${formatBytes(MAX_AUDIO_UPLOAD_BYTES)}。`}
        </p>
        {file ? (
          <p className="text-sm text-text-muted">
            已选择：<span className="font-medium text-text">{file.name}</span>（
            {formatBytes(file.size)}）
          </p>
        ) : null}
        {fileError ? (
          <p id="upload-import-file-error" className="text-sm text-danger">
            {fileError}
          </p>
        ) : null}
      </div>

      {audioOnly ? (
        <p className="rounded-md bg-info-bg px-3 py-2 text-sm text-text-muted">
          先将录音转为原文，再由 AI
          生成统一格式的总结、学生笔记和教师教纲。原文会保留在课时中，可继续生成知识点与测验。
        </p>
      ) : null}
      <div className="grid gap-4 sm:grid-cols-2">
        {!audioOnly ? (
          <div className="space-y-1.5">
            <Label htmlFor="upload-import-transcription">转写方式</Label>
            <select
              id="upload-import-transcription"
              value={transcriptionMode}
              onChange={(event) => setTranscriptionMode(event.target.value)}
              className="h-9 w-full rounded-md border border-border-strong bg-surface px-3 text-sm text-text"
            >
              {TRANSCRIPTION_MODES.map((mode) => (
                <option key={mode.value} value={mode.value}>
                  {mode.label}
                </option>
              ))}
            </select>
          </div>
        ) : null}
        <div className="space-y-1.5">
          <Label htmlFor="upload-import-language">{audioOnly ? '音频语言' : '语言偏好'}</Label>
          {audioOnly ? (
            <select
              id="upload-import-language"
              value={preferredLanguage}
              disabled={pending}
              onChange={(event) => setPreferredLanguage(event.target.value)}
              className="h-9 w-full rounded-md border border-border-strong bg-surface px-3 text-sm text-text"
            >
              {AUDIO_LANGUAGES.map((language) => (
                <option key={language.value} value={language.value}>
                  {language.label}
                </option>
              ))}
            </select>
          ) : (
            <input
              id="upload-import-language"
              type="text"
              value={preferredLanguage}
              onChange={(event) => setPreferredLanguage(event.target.value)}
              placeholder="可选，如 zh、en"
              className="h-9 w-full rounded-md border border-border-strong bg-surface px-3 text-sm text-text placeholder:text-text-subtle"
            />
          )}
        </div>
      </div>

      {pending ? (
        <div aria-live="polite">
          <div className="flex items-center justify-between text-sm">
            <span className="font-medium text-text">上传进度</span>
            <span className="text-text-muted">
              {progress?.percent !== undefined
                ? `${progress.percent}%`
                : progress
                  ? `已上传 ${formatBytes(progress.loaded)}`
                  : '准备上传…'}
            </span>
          </div>
          <div
            role="progressbar"
            aria-valuenow={progress?.percent}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label="文件上传进度"
            className="mt-1 h-1.5 w-full rounded-full bg-surface-sunken"
          >
            <div
              className="h-full rounded-full bg-info transition-[width] duration-180"
              style={{ width: `${progress?.percent ?? 0}%` }}
            />
          </div>
          <p className="mt-1 text-xs text-text-subtle">
            上传完成后会创建后台生成任务，任务进度在课时页与任务中心展示。
          </p>
        </div>
      ) : null}

      <Button type="submit" size="sm" disabled={pending}>
        {pending ? '上传中…' : audioOnly ? '上传音频并生成内容' : '上传并导入'}
      </Button>
    </form>
  )
}
