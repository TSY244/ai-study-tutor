import type {
  AttemptStatus,
  DocumentKind,
  JobKind,
  JobStatus,
  LessonStatus,
  QuizStatus,
} from '@/api/generated/model'

import type { BadgeVariant } from '@/components/ui/badge'

/**
 * 状态矩阵：领域状态 -> 文案 + Badge 变体 + 是否进行中。
 * 状态绝不只靠颜色表达——Badge 永远带文本标签。可用操作（重试/取消等）
 * 在 F3 按这里的状态判断接入，不在本层定义。
 */
export interface StatusMeta {
  label: string
  variant: BadgeVariant
}

export const JOB_STATUS_META: Record<JobStatus, StatusMeta> = {
  queued: { label: '排队中', variant: 'neutral' },
  running: { label: '运行中', variant: 'info' },
  validating: { label: '校验中', variant: 'info' },
  succeeded: { label: '已成功', variant: 'ok' },
  failed: { label: '失败', variant: 'danger' },
  cancel_requested: { label: '取消中', variant: 'warn' },
  cancelled: { label: '已取消', variant: 'neutral' },
}

/** 进行中的任务：任务中心角标与 active 过滤只关心这些状态。 */
export function isJobActive(status: JobStatus): boolean {
  return (
    status === 'queued' ||
    status === 'running' ||
    status === 'validating' ||
    status === 'cancel_requested'
  )
}

export const JOB_KIND_LABEL: Record<JobKind, string> = {
  claude_connectivity_test: 'Claude 连通性测试',
  lesson_import: '课时导入',
  lesson_regenerate: '课时重新生成',
  quiz_generate: '测验生成',
  review_quiz_generate: '复习测验生成',
  attempt_grade: '作答批阅',
  wrong_answer_analyze: '错题分析',
  summary_evaluate: '总结评估',
  lesson_export: '课时导出',
  bilibili_cookie_test: 'B 站 Cookie 检测',
}

export const LESSON_STATUS_META: Record<LessonStatus, StatusMeta> = {
  draft: { label: '草稿', variant: 'neutral' },
  processing: { label: '生成中', variant: 'info' },
  ready: { label: '就绪', variant: 'ok' },
  failed: { label: '生成失败', variant: 'danger' },
  ready_with_warning: { label: '部分就绪', variant: 'warn' },
}

export const ATTEMPT_STATUS_META: Record<AttemptStatus, StatusMeta> = {
  in_progress: { label: '作答中', variant: 'info' },
  submitted: { label: '已提交', variant: 'neutral' },
  grading: { label: '批阅中', variant: 'info' },
  graded: { label: '已批阅', variant: 'ok' },
  grading_failed: { label: '批阅失败', variant: 'danger' },
}

export const QUIZ_STATUS_META: Record<QuizStatus, StatusMeta> = {
  draft: { label: '草稿', variant: 'neutral' },
  ready: { label: '可用', variant: 'ok' },
  archived: { label: '已归档', variant: 'neutral' },
}

/** 文档类型标签（课时笔记/原文/教学大纲）。 */
export const DOCUMENT_KIND_LABEL: Record<DocumentKind, string> = {
  raw_text: '原文',
  student_notes: '笔记',
  teacher_outline: '大纲',
}

/** 系统组件状态（system status 端点）：ok/degraded/unavailable/unknown。 */
export const SYSTEM_STATUS_META: Record<string, StatusMeta> = {
  ok: { label: '正常', variant: 'ok' },
  degraded: { label: '降级', variant: 'warn' },
  unavailable: { label: '不可用', variant: 'danger' },
  unknown: { label: '未知', variant: 'neutral' },
}

export function systemStatusMeta(status: string): StatusMeta {
  return SYSTEM_STATUS_META[status] ?? { label: status, variant: 'neutral' }
}
