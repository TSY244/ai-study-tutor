import { formatBytes } from '@/lib/upload-limits'

export const AUDIO_ACCEPT = 'audio/*,.mp3,.wav,.m4a,.flac,.ogg,.opus,.aac'
export const MAX_AUDIO_UPLOAD_BYTES = 2 * 1024 * 1024 * 1024
export const AUDIO_FORMATS = 'MP3、WAV、M4A、FLAC、OGG、OPUS、AAC'
export const AUDIO_LANGUAGES = [
  { value: '', label: '自动识别' },
  { value: 'zh', label: '中文' },
  { value: 'en', label: '英语' },
  { value: 'ja', label: '日语' },
  { value: 'ko', label: '韩语' },
] as const

export function isAudioFile(file: File): boolean {
  return /\.(mp3|wav|m4a|flac|ogg|opus|aac)$/i.test(file.name)
}

export function validateAudioFile(file: File): string | null {
  if (!isAudioFile(file)) return `请选择音频文件，支持 ${AUDIO_FORMATS}。`
  if (file.size === 0) return '音频文件为空，请重新选择。'
  if (file.size > MAX_AUDIO_UPLOAD_BYTES)
    return `音频超过 ${formatBytes(MAX_AUDIO_UPLOAD_BYTES)}，请压缩或分段上传。`
  return null
}
