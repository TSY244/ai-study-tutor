import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'

import { recordDiagnostic } from '@/api/diagnostics'
import { isApiProblemError } from '@/api/problem'

/**
 * 服务端状态唯一事实来源。业务实体不得复制到全局客户端 Store。
 *
 * 重试策略：4xx 业务错误（Problem）不重试；5xx 与 network_error 最多重试 2 次，
 * 带抖动退避。Mutation 不自动重试——创建/任务型请求由调用方携带
 * Idempotency-Key 并按 code（idempotency_in_progress 等）显式处理。
 */
export function createAppQueryClient(): QueryClient {
  const onError = (error: Error) => {
    if (isApiProblemError(error)) {
      recordDiagnostic({
        requestId: error.requestId,
        method: '-',
        url: '(query/mutation)',
        status: error.status,
        code: error.code,
        durationMs: 0,
        at: new Date().toISOString(),
      })
    }
  }

  return new QueryClient({
    queryCache: new QueryCache({ onError }),
    mutationCache: new MutationCache({ onError }),
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        gcTime: 5 * 60_000,
        retry: (failureCount, error) => {
          if (isApiProblemError(error)) {
            const retryable = error.status >= 500 || error.code === 'network_error'
            return retryable && failureCount < 2
          }
          return failureCount < 2
        },
        retryDelay: (attempt) => Math.min(1000 * 2 ** attempt + Math.random() * 250, 10_000),
      },
      mutations: {
        retry: false,
      },
    },
  })
}

/**
 * 资源 Query Key 约定（02-frontend-handoff.md §5.1）。
 * 集中定义，避免各页面拼写漂移；值即 Query Cache 的失效边界。
 */
export const queryKeys = {
  systemStatus: () => ['system', 'status'] as const,
  courses: (filters?: Record<string, unknown>) => ['courses', filters ?? {}] as const,
  course: (courseId: string) => ['course', courseId] as const,
  lessons: (courseId: string) => ['lessons', courseId] as const,
  lesson: (lessonId: string) => ['lesson', lessonId] as const,
  /** 当前文档集合（DocumentSummaryCollection，每种 kind 一项）。 */
  documents: (lessonId: string) => ['documents', lessonId] as const,
  /** 指定 kind 的当前文档内容。 */
  document: (lessonId: string, kind: string) => ['document', lessonId, kind] as const,
  /** 指定 kind 的历史版本列表（游标分页，infinite query 追加视图参数）。 */
  documentVersions: (lessonId: string, kind: string) =>
    ['document-versions', lessonId, kind] as const,
  /** 单个历史版本内容（只读快照）。 */
  documentVersion: (documentId: string) => ['document-version', documentId] as const,
  quizzes: (lessonId: string) => ['quizzes', lessonId] as const,
  quiz: (quizId: string) => ['quiz', quizId] as const,
  attempt: (attemptId: string) => ['attempt', attemptId] as const,
  jobs: (filters?: Record<string, unknown>) => ['jobs', filters ?? {}] as const,
  job: (jobId: string) => ['job', jobId] as const,
  /** 任务事件时间线（REST 审计 JobEvent，游标分页，infinite query）。 */
  jobEvents: (jobId: string) => ['job-events', jobId] as const,
  weaknesses: (filters?: Record<string, unknown>) => ['weaknesses', filters ?? {}] as const,
  progress: (courseId: string) => ['progress', courseId] as const,
  progressLesson: (lessonId: string) => ['progress', 'lesson', lessonId] as const,
}
