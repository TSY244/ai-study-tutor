import { http, HttpResponse } from 'msw'

import { getAttemptsMock } from '@/api/generated/endpoints/attempts/attempts.msw'
import { getBackupsMock } from '@/api/generated/endpoints/backups/backups.msw'
import { getCoursesMock } from '@/api/generated/endpoints/courses/courses.msw'
import { getDocumentsMock } from '@/api/generated/endpoints/documents/documents.msw'
import { getExportsMock } from '@/api/generated/endpoints/exports/exports.msw'
import { getFilesMock } from '@/api/generated/endpoints/files/files.msw'
import { getJobsMock } from '@/api/generated/endpoints/jobs/jobs.msw'
import { getLessonsMock } from '@/api/generated/endpoints/lessons/lessons.msw'
import { getMaterialsMock } from '@/api/generated/endpoints/materials/materials.msw'
import { getProgressMock } from '@/api/generated/endpoints/progress/progress.msw'
import { getQuizzesMock } from '@/api/generated/endpoints/quizzes/quizzes.msw'
import { getSummariesMock } from '@/api/generated/endpoints/summaries/summaries.msw'
import { getSystemMock } from '@/api/generated/endpoints/system/system.msw'
import { getWeaknessesMock } from '@/api/generated/endpoints/weaknesses/weaknesses.msw'
import type {
  AnswerSave,
  AsyncAccepted,
  Attempt,
  AttemptAnswer,
  AttemptSubmitAccepted,
  AttemptSubmitCompleted,
  AttemptSummary,
  ClaudeKey,
  ClaudeKeyCreate,
  ClaudeKeyUpdate,
  Course,
  CourseCreate,
  CourseUpdate,
  CreateSummaryEvaluationBody,
  Document,
  DocumentSummary,
  FieldError,
  Job,
  JobStreamData,
  Lesson,
  LessonCreate,
  LessonUpdate,
  Material,
  MaterialJobAccepted,
  Question,
  Quiz,
  QuizGenerationCreate,
  QuizSummary,
  SavedAnswer,
  SubmitAttemptBody,
  SummaryEvaluation,
  SummaryEvaluationAccepted,
  TextMaterialCreate,
  UrlMaterialCreate,
} from '@/api/generated/model'
import {
  demoAttempt,
  demoAttemptFailed,
  demoAttemptFailedWithJob,
  demoAttemptGraded,
  demoClaudeKeys,
  demoCourses,
  demoDocuments,
  demoJobEvents,
  demoJobs,
  demoLessons,
  demoObjectiveQuiz,
  demoProgressByCourse,
  demoProgressByLesson,
  demoQuiz,
  demoSummaryEvaluationFeedback,
  demoWeaknesses,
  QUIZ_ANSWER_KEYS,
  systemStatusOk,
} from '@/mocks/fixtures'
import { createJobStreamMock } from '@/mocks/job-stream-mock'
import { answerValuesEqual, isObjectiveQuestion } from '@/features/attempts/answer-value'
import { isJobActive } from '@/lib/status'
import { MAX_UPLOAD_BYTES } from '@/lib/upload-limits'

/**
 * MSW 接线（可替换边界）：
 * - curated handlers 在前：为开发/测试提供确定性响应，模拟服务端关键语义——
 *   幂等创建（同 Key 重放返回首响应，异载荷判 idempotency_conflict）、
 *   乐观锁（version 失配判 version_conflict）、resource_busy（一课时就一个
 *   活动生成任务）、乐观分页（cursor=偏移，前端按不透明游标对待）、
 *   并模拟中间件始终返回 X-Request-ID 的行为（回显请求头）。
 * - orval 生成的 faker handlers 在后兜底：覆盖全部端点，契约发布后
 *   随 `npm run api:gen` 自动更新，无需手改。
 * - SSE（/api/v1/jobs/stream）：前端客户端是 fetch 传输（非 EventSource），
 *   Service Worker 可拦截；流 mock 由 createJobStreamMock 提供常驻流，
 *   浏览器接线时把控制器挂到 window.__jobStreamMock 供 E2E 注入事件。
 */

function echoRequestId(request: Request): HeadersInit {
  return { 'X-Request-ID': request.headers.get('X-Request-ID') ?? 'mock-request-id' }
}

function json(
  request: Request,
  body: Parameters<typeof HttpResponse.json>[0],
  init?: ResponseInit,
) {
  return HttpResponse.json(body, {
    ...init,
    headers: { ...echoRequestId(request), ...init?.headers },
  })
}

function problem(
  request: Request,
  init: {
    status: number
    code: string
    title: string
    detail: string
    errors?: FieldError[]
    extensions?: Record<string, unknown>
  },
) {
  return HttpResponse.json(
    {
      type: 'about:blank',
      title: init.title,
      status: init.status,
      detail: init.detail,
      code: init.code,
      ...(init.errors ? { errors: init.errors } : {}),
      ...init.extensions,
    },
    {
      status: init.status,
      headers: {
        'Content-Type': 'application/problem+json',
        ...echoRequestId(request),
      },
    },
  )
}

function notFound(request: Request, detail: string) {
  return problem(request, { status: 404, code: 'not_found', title: '资源不存在', detail })
}

function validationProblem(request: Request, errors: FieldError[]) {
  return problem(request, {
    status: 422,
    code: 'validation_error',
    title: '表单未通过校验',
    detail: '字段不符合契约约束。',
    errors,
  })
}

// ---------------------------------------------------------------------------
// 可重置状态：curated fixtures 是只读常量；运行时变化全部落在这里。
// ---------------------------------------------------------------------------

const createdCourses: Course[] = []
const createdLessons: Lesson[] = []
const createdMaterials: Material[] = []
const createdJobs: Job[] = []
const claudeKeysStore: ClaudeKey[] = []
const courseOverrides = new Map<string, Course>()
const lessonOverrides = new Map<string, Lesson>()
const jobOverrides = new Map<string, Job>()
const deletedCourseIds = new Set<string>()
const deletedLessonIds = new Set<string>()
const idempotencyLog = new Map<string, { fingerprint: string; response: unknown }>()

// 作答运行时状态：Attempt 会被 autosave/提交/批阅推进改变，存可变副本。
const attemptsStore = new Map<string, Attempt>()
/** grading 状态的 GET 计数：模拟 Worker 在后台完成批阅（第 2 次查询起返回 graded）。 */
const attemptGetCounts = new Map<string, number>()

// 总结评估运行时状态：评估会被 Worker 推进改变，存可变副本。
const summaryEvaluationsStore = new Map<string, SummaryEvaluation>()
/** 评估 id → 评估任务 id（推进时联动完成任务）。 */
const summaryEvaluationJobs = new Map<string, string>()
/** evaluating 状态的 GET 计数：模拟 Worker 在后台完成评估（第 2 次查询起返回 succeeded）。 */
const summaryEvaluationGetCounts = new Map<string, number>()

function seedAttempts(): void {
  attemptsStore.clear()
  attemptGetCounts.clear()
  for (const attempt of [demoAttempt, demoAttemptFailed, demoAttemptFailedWithJob, demoAttemptGraded]) {
    attemptsStore.set(attempt.id, structuredClone(attempt))
  }
}
seedAttempts()

function seedClaudeKeys(): void {
  claudeKeysStore.length = 0
  claudeKeysStore.push(...demoClaudeKeys.map((key) => structuredClone(key)))
}
seedClaudeKeys()

/** 测试隔离：清空全部运行时状态（setup.ts afterEach 调用）。 */
export function resetMockState(): void {
  createdCourses.length = 0
  createdLessons.length = 0
  createdMaterials.length = 0
  createdJobs.length = 0
  courseOverrides.clear()
  lessonOverrides.clear()
  jobOverrides.clear()
  deletedCourseIds.clear()
  deletedLessonIds.clear()
  idempotencyLog.clear()
  summaryEvaluationsStore.clear()
  summaryEvaluationJobs.clear()
  summaryEvaluationGetCounts.clear()
  seedAttempts()
  seedClaudeKeys()
}

function nowIso(): string {
  return new Date().toISOString()
}

function baseCourses(): Course[] {
  return [...createdCourses, ...demoCourses]
    .filter((course) => !deletedCourseIds.has(course.id))
    .map((course) => courseOverrides.get(course.id) ?? course)
}

function baseLessons(): Lesson[] {
  return [...createdLessons, ...demoLessons]
    .filter((lesson) => !deletedLessonIds.has(lesson.id))
    .map((lesson) => lessonOverrides.get(lesson.id) ?? lesson)
}

function baseJobs(): Job[] {
  return [...createdJobs, ...demoJobs].map((job) => jobOverrides.get(job.id) ?? job)
}

function withLessonCount(course: Course): Course {
  return {
    ...course,
    lesson_count: baseLessons().filter((lesson) => lesson.course_id === course.id).length,
  }
}

function findCourse(id: string): Course | undefined {
  return baseCourses()
    .map(withLessonCount)
    .find((course) => course.id === id)
}

function findLesson(id: string): Lesson | undefined {
  return baseLessons().find((lesson) => lesson.id === id)
}

function findJob(id: string): Job | undefined {
  return baseJobs().find((job) => job.id === id)
}

function toDocumentSummary(doc: Document): DocumentSummary {
  const { content_md: _content, ...summary } = doc
  return summary
}

/** 当前发布文档：每个 DocumentKind 取 is_active 的最新 revision。 */
function activeDocumentsOf(lessonId: string): Document[] {
  const byKind = new Map<string, Document>()
  for (const doc of demoDocuments) {
    if (doc.lesson_id !== lessonId || !doc.is_active) continue
    const existing = byKind.get(doc.kind)
    if (!existing || doc.revision > existing.revision) byKind.set(doc.kind, doc)
  }
  return [...byKind.values()]
}

function patchCourse(id: string, patch: Partial<Course>): void {
  const current = baseCourses().find((course) => course.id === id)
  if (current) courseOverrides.set(id, { ...current, ...patch, updated_at: nowIso() })
}

function patchLesson(id: string, patch: Partial<Lesson>): void {
  const current = baseLessons().find((lesson) => lesson.id === id)
  if (current) lessonOverrides.set(id, { ...current, ...patch, updated_at: nowIso() })
}

function patchJob(id: string, patch: Partial<Job>): void {
  const current = baseJobs().find((job) => job.id === id)
  if (current) jobOverrides.set(id, { ...current, ...patch, updated_at: nowIso() })
}

// ---------------------------------------------------------------------------
// 幂等语义：同 Key 同载荷重放首响应；同 Key 异载荷判 idempotency_conflict。
// ---------------------------------------------------------------------------

function idempotencyConflictProblem(request: Request) {
  return problem(request, {
    status: 409,
    code: 'idempotency_conflict',
    title: '请求标识冲突',
    detail: '同一 Idempotency-Key 携带了不同的请求内容。',
  })
}

function replayIdempotent<T>(
  request: Request,
  fingerprint: string,
): { replay: T } | { conflict: true } | null {
  const key = request.headers.get('Idempotency-Key')
  if (!key) return null
  const seen = idempotencyLog.get(key)
  if (!seen) return null
  return seen.fingerprint === fingerprint ? { replay: seen.response as T } : { conflict: true }
}

function rememberIdempotent(request: Request, fingerprint: string, response: unknown): void {
  const key = request.headers.get('Idempotency-Key')
  if (key) idempotencyLog.set(key, { fingerprint, response })
}

// ---------------------------------------------------------------------------
// 游标分页：cursor 是不透明偏移量（前端只透传 page_info.next_cursor）。
// ---------------------------------------------------------------------------

function paginate<T>(
  items: T[],
  url: URL,
): { items: T[]; page_info: { next_cursor: string | null; has_more: boolean } } {
  const limit = Math.min(Math.max(Number(url.searchParams.get('limit')) || 20, 1), 100)
  const offset = Math.max(Number(url.searchParams.get('cursor')) || 0, 0)
  const next = offset + limit
  return {
    items: items.slice(offset, next),
    page_info: {
      next_cursor: next < items.length ? String(next) : null,
      has_more: next < items.length,
    },
  }
}

// ---------------------------------------------------------------------------
// 校验与工厂
// ---------------------------------------------------------------------------

function validateCourseFields(body: Partial<CourseCreate> | null): FieldError[] {
  const fieldErrors: FieldError[] = []
  if (!body || typeof body.title !== 'string' || body.title.trim().length === 0) {
    fieldErrors.push({ field: 'title', code: 'required', message: '请输入课程标题。' })
  } else if (body.title.length > 200) {
    fieldErrors.push({ field: 'title', code: 'too_long', message: '课程标题最长 200 字。' })
  }
  if (typeof body?.description === 'string' && body.description.length > 5000) {
    fieldErrors.push({ field: 'description', code: 'too_long', message: '课程描述最长 5000 字。' })
  }
  return fieldErrors
}

function validateLessonFields(body: Partial<LessonCreate> | null): FieldError[] {
  const fieldErrors: FieldError[] = []
  if (!body || !Number.isInteger(body.ordinal) || (body.ordinal as number) < 1) {
    fieldErrors.push({ field: 'ordinal', code: 'invalid', message: '课次必须是 ≥1 的整数。' })
  }
  if (!body || typeof body.title !== 'string' || body.title.trim().length === 0) {
    fieldErrors.push({ field: 'title', code: 'required', message: '请输入课时标题。' })
  } else if (body.title.length > 200) {
    fieldErrors.push({ field: 'title', code: 'too_long', message: '课时标题最长 200 字。' })
  }
  if (typeof body?.description === 'string' && body.description.length > 5000) {
    fieldErrors.push({ field: 'description', code: 'too_long', message: '课时描述最长 5000 字。' })
  }
  return fieldErrors
}

function newImportJob(lessonId: string): Job {
  const now = nowIso()
  return {
    id: globalThis.crypto.randomUUID(),
    kind: 'lesson_import',
    aggregate_type: 'lesson',
    aggregate_id: lessonId,
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
  }
}

function newQuizGenerationJob(lessonId: string): Job {
  const now = nowIso()
  return {
    id: globalThis.crypto.randomUUID(),
    kind: 'quiz_generate',
    aggregate_type: 'lesson',
    aggregate_id: lessonId,
    generation_family: 'lesson_generation',
    retry_of_job_id: null,
    status: 'queued',
    stage: '排队中',
    progress: 0,
    message: null,
    attempt_count: 1,
    max_attempts: 3,
    retryable: false,
    cancel_requested: false,
    error: null,
    created_at: now,
    started_at: null,
    completed_at: null,
    updated_at: now,
  }
}

function newConnectivityJob(): Job {
  const now = nowIso()
  return {
    id: globalThis.crypto.randomUUID(),
    kind: 'claude_connectivity_test',
    aggregate_type: 'system',
    aggregate_id: null,
    generation_family: null,
    retry_of_job_id: null,
    status: 'queued',
    stage: '排队中',
    progress: 0,
    message: null,
    attempt_count: 0,
    max_attempts: 3,
    retryable: false,
    cancel_requested: false,
    error: null,
    created_at: now,
    started_at: null,
    completed_at: null,
    updated_at: now,
  }
}

function maskGatewaySecret(secret: string): string {
  return secret.length <= 8 ? '...' : `${secret.slice(0, 7)}...${secret.slice(-4)}`
}

async function fingerprintGatewaySecret(secret: string): Promise<string> {
  const digest = await globalThis.crypto.subtle.digest(
    'SHA-256',
    new TextEncoder().encode(secret.trim()),
  )
  return `sha256:${Array.from(new Uint8Array(digest).slice(0, 6), (byte) =>
    byte.toString(16).padStart(2, '0'),
  ).join('')}`
}

// ---------------------------------------------------------------------------
// 作答（Attempt）：answer_version 乐观锁、提交判分、批阅推进
// ---------------------------------------------------------------------------

function baseQuizzes(): Quiz[] {
  return [demoQuiz, demoObjectiveQuiz]
}

function findQuiz(id: string): Quiz | undefined {
  return baseQuizzes().find((quiz) => quiz.id === id)
}

function toQuizSummary(quiz: Quiz): QuizSummary {
  return {
    id: quiz.id,
    lesson_id: quiz.lesson_id,
    title: quiz.title,
    source: quiz.source,
    question_count: quiz.questions.length,
    created_at: quiz.created_at,
  }
}

function findAttempt(id: string): Attempt | undefined {
  return attemptsStore.get(id)
}

/** 作答 → 历史列表摘要（契约 AttemptSummary；不含题目/答案重型字段）。 */
function toAttemptSummary(attempt: Attempt): AttemptSummary {
  return {
    id: attempt.id,
    quiz_id: attempt.quiz_id,
    quiz_title: findQuiz(attempt.quiz_id)?.title ?? '测验',
    status: attempt.status,
    auto_score: attempt.auto_score ?? null,
    final_score: attempt.final_score ?? null,
    max_score: attempt.max_score ?? null,
    started_at: attempt.started_at,
    submitted_at: attempt.submitted_at ?? null,
    graded_at: attempt.graded_at ?? null,
  }
}

function newGradingJob(attempt: Attempt): Job {
  const now = nowIso()
  return {
    id: globalThis.crypto.randomUUID(),
    kind: 'attempt_grade',
    aggregate_type: 'attempt',
    aggregate_id: attempt.id,
    generation_family: null,
    retry_of_job_id: attempt.grading_job_id,
    status: 'queued',
    stage: '排队中',
    progress: 0,
    message: null,
    attempt_count: 1,
    max_attempts: 3,
    retryable: false,
    cancel_requested: false,
    error: null,
    created_at: now,
    started_at: null,
    completed_at: null,
    updated_at: now,
  }
}

/** 客观题判分：对照 QUIZ_ANSWER_KEYS 写回 awarded_score/is_correct。 */
function gradeObjectiveAnswers(attempt: Attempt): number {
  const keys = QUIZ_ANSWER_KEYS[attempt.quiz_id] ?? {}
  let autoScore = 0
  for (const question of attempt.questions) {
    if (!isObjectiveQuestion(question.type)) continue
    const answer = attempt.answers.find((item) => item.question_id === question.id)
    const key = keys[question.id]
    const correct =
      answer !== undefined && key !== undefined && answerValuesEqual(answer.answer.value, key)
    const awarded = correct ? question.score : 0
    autoScore += awarded
    if (answer) {
      answer.awarded_score = awarded
      answer.is_correct = correct
      answer.feedback = {
        question_id: question.id,
        awarded_score: awarded,
        max_score: question.score,
        is_correct: correct,
        feedback: correct ? '回答正确。' : '回答错误。',
        correct_understanding: `本题考查：${question.tested_concepts.join('、')}。`,
        correct_answer: key === undefined ? null : { value: key },
        explanation: null,
        weak_concepts: correct ? [] : [...question.tested_concepts],
      }
    }
  }
  return autoScore
}

/** 批阅推进：grading 的 Attempt 第 2 次被查询时判为批阅完成（模拟 Worker）。 */
function advanceGrading(attempt: Attempt): void {
  for (const question of attempt.questions) {
    if (isObjectiveQuestion(question.type)) continue
    const answer = attempt.answers.find((item) => item.question_id === question.id)
    if (!answer) continue
    answer.awarded_score = question.score
    answer.is_correct = true
    answer.feedback = {
      question_id: question.id,
      awarded_score: question.score,
      max_score: question.score,
      is_correct: true,
      feedback: '要点完整，表述清晰。',
      correct_understanding: `本题考查：${question.tested_concepts.join('、')}。`,
      correct_answer: null,
      explanation: '参考答案需覆盖期望与方差两个公式，并说明适用条件。',
      weak_concepts: [],
    }
  }
  const subjectiveScore = attempt.questions
    .filter((question) => !isObjectiveQuestion(question.type))
    .reduce((total, question) => {
      const answer = attempt.answers.find((item) => item.question_id === question.id)
      return total + (answer?.awarded_score ?? 0)
    }, 0)
  attempt.final_score = (attempt.auto_score ?? 0) + subjectiveScore
  attempt.status = 'graded'
  attempt.graded_at = nowIso()
  if (attempt.grading_job_id) {
    patchJob(attempt.grading_job_id, {
      status: 'succeeded',
      stage: '完成',
      progress: 100,
      message: '批阅完成',
      completed_at: nowIso(),
    })
  }
}

/** 总结评估任务：kind summary_evaluate，聚合到评估本身（SSE 终态对账按此失效评估查询）。 */
function newSummaryEvaluationJob(evaluationId: string): Job {
  const now = nowIso()
  return {
    id: globalThis.crypto.randomUUID(),
    kind: 'summary_evaluate',
    aggregate_type: 'summary_evaluation',
    aggregate_id: evaluationId,
    generation_family: null,
    retry_of_job_id: null,
    status: 'queued',
    stage: '排队中',
    progress: 0,
    message: null,
    attempt_count: 1,
    max_attempts: 3,
    retryable: false,
    cancel_requested: false,
    error: null,
    created_at: now,
    started_at: null,
    completed_at: null,
    updated_at: now,
  }
}

/** 评估推进：evaluating 的评估第 2 次被查询时写入固定反馈（模拟 Worker）。 */
function advanceSummaryEvaluation(evaluation: SummaryEvaluation): void {
  evaluation.status = 'succeeded'
  evaluation.coverage_score = demoSummaryEvaluationFeedback.coverage_score
  evaluation.correct_points = [...demoSummaryEvaluationFeedback.correct_points]
  evaluation.misconceptions = [...demoSummaryEvaluationFeedback.misconceptions]
  evaluation.missing_points = [...demoSummaryEvaluationFeedback.missing_points]
  evaluation.suggestions = [...demoSummaryEvaluationFeedback.suggestions]
  evaluation.completed_at = nowIso()
  const jobId = summaryEvaluationJobs.get(evaluation.id)
  if (jobId) {
    patchJob(jobId, {
      status: 'succeeded',
      stage: '完成',
      progress: 100,
      message: '评估完成',
      completed_at: nowIso(),
    })
  }
}

/** 素材导入公共路径：幂等重放 -> resource_busy -> 接受并建任务。 */
function acceptMaterialImport(
  request: Request,
  lesson: Lesson,
  material: Material,
  fingerprint: string,
) {
  const replay = replayIdempotent<MaterialJobAccepted>(request, fingerprint)
  if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
  if (replay) return json(request, replay.replay, { status: 202 })

  if (lesson.status === 'processing' && lesson.active_job_id) {
    return problem(request, {
      status: 409,
      code: 'resource_busy',
      title: '已有任务在进行',
      detail: '同一课时同时只允许一个生成任务，请等待现有任务完成。',
      extensions: { active_job_id: lesson.active_job_id, generation_family: 'lesson_generation' },
    })
  }

  const job = newImportJob(lesson.id)
  createdJobs.unshift(job)
  createdMaterials.unshift(material)
  patchLesson(lesson.id, { status: 'processing', active_job_id: job.id })
  const accepted: MaterialJobAccepted = { material, job }
  rememberIdempotent(request, fingerprint, accepted)
  return json(request, accepted, { status: 202 })
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

/** 任务事件流 mock 单例：浏览器接线把 controller 挂到 window 供 E2E 推送。 */
const jobStreamMock = createJobStreamMock()

/**
 * 流事件 → REST 状态桥（模拟 Worker 写库）：推送的持久化事件同步进
 * 任务存储；活动批阅任务收到 job.succeeded 时由 advanceGrading 完成
 * 对应 Attempt 判分。保证「SSE 推送 → 客户端失效 → REST 重取」
 * 三处状态一致，事件时间线与任务列表不出现回跳。
 */
jobStreamMock.controller.onPush((frame) => {
  if (!frame.event.startsWith('job.')) return
  const data = frame.data as Partial<JobStreamData> | null
  if (!data || typeof data.job_id !== 'string' || !data.status) return
  const job = findJob(data.job_id)
  if (!job) return
  if (
    frame.event === 'job.succeeded' &&
    job.kind === 'attempt_grade' &&
    job.aggregate_id &&
    isJobActive(job.status)
  ) {
    const attempt = attemptsStore.get(job.aggregate_id)
    if (attempt && attempt.status === 'grading') advanceGrading(attempt)
    return
  }
  patchJob(job.id, {
    status: data.status,
    stage: data.stage ?? job.stage,
    progress: data.progress ?? job.progress,
    message: data.message ?? null,
    cancel_requested: data.status === 'cancel_requested' ? true : job.cancel_requested,
    ...(isJobActive(data.status) ? {} : { completed_at: nowIso() }),
  })
  // 课时聚合任务终态 → 课时状态联动（模拟 Worker 写库：status/active_job_id）
  if (
    (frame.event === 'job.succeeded' || frame.event === 'job.failed') &&
    job.aggregate_type === 'lesson' &&
    job.aggregate_id
  ) {
    const lesson = findLesson(job.aggregate_id)
    if (lesson && lesson.active_job_id === job.id) {
      patchLesson(lesson.id, {
        status: frame.event === 'job.succeeded' ? 'ready' : 'failed',
        active_job_id: null,
      })
    }
  }
})

/** E2E/调试窥视：当前任务存储快照（只读副本，定位服务端创建的 Job id）。 */
export function __testPeekJobs(): Job[] {
  return baseJobs().map((job) => ({ ...job }))
}

export const jobStreamMockController = jobStreamMock.controller

const curatedHandlers = [
  http.get('*/healthz', ({ request }) => json(request, { status: 'ok' })),
  http.get('*/readyz', ({ request }) => json(request, { status: 'ok' })),
  http.get('*/api/v1/system/status', ({ request }) => json(request, systemStatusOk)),

  // Claude 多 Key 网关：只保存/返回安全视图，完整 Key 仅在单次请求处理栈中存在。
  http.get('*/api/v1/system/claude-keys', ({ request }) =>
    json(request, {
      items: claudeKeysStore.map((key) => ({ ...key })),
      routing: 'priority_failover',
      legacy_fallback: false,
      write_enabled: true,
    }),
  ),
  http.post('*/api/v1/system/claude-keys', async ({ request }) => {
    const body = (await request.json().catch(() => null)) as ClaudeKeyCreate | null
    if (!body || !body.api_key?.trim() || !body.name?.trim() || !body.model?.trim()) {
      return validationProblem(request, [
        { field: 'api_key', code: 'required', message: '请输入 Anthropic API Key。' },
      ])
    }
    const fingerprint = await fingerprintGatewaySecret(body.api_key)
    const requestFingerprint = JSON.stringify({ ...body, api_key: fingerprint })
    const replay = replayIdempotent<ClaudeKey>(request, requestFingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 201 })

    const now = nowIso()
    const key: ClaudeKey = {
      id: `ck_${globalThis.crypto.randomUUID().replaceAll('-', '').slice(0, 16)}`,
      name: body.name.trim(),
      masked_key: maskGatewaySecret(body.api_key.trim()),
      fingerprint,
      model: body.model.trim(),
      enabled: body.enabled ?? true,
      priority: body.priority ?? 10,
      weight: body.weight ?? 100,
      timeout_seconds: body.timeout_seconds ?? 300,
      cooldown_seconds: body.cooldown_seconds ?? 300,
      max_consecutive_failures: body.max_consecutive_failures ?? 2,
      status: body.enabled === false ? 'disabled' : 'unknown',
      consecutive_failures: 0,
      last_error_code: null,
      last_success_at: null,
      last_failure_at: null,
      cooldown_until: null,
      last_latency_ms: null,
      created_at: now,
      updated_at: now,
    }
    claudeKeysStore.push(key)
    rememberIdempotent(request, requestFingerprint, key)
    return json(request, key, { status: 201 })
  }),
  http.patch('*/api/v1/system/claude-keys/:keyId', async ({ request, params }) => {
    const index = claudeKeysStore.findIndex((key) => key.id === params.keyId)
    if (index < 0) return notFound(request, 'Claude Key 不存在或已删除。')
    const body = (await request.json().catch(() => null)) as ClaudeKeyUpdate | null
    if (!body) return validationProblem(request, [])
    const current = claudeKeysStore[index] as ClaudeKey
    let maskedKey = current.masked_key
    let fingerprint = current.fingerprint
    if (body.api_key !== undefined) {
      maskedKey = maskGatewaySecret(body.api_key.trim())
      fingerprint = await fingerprintGatewaySecret(body.api_key)
    }
    const enabled = body.enabled ?? current.enabled
    const { api_key: _discardedSecret, ...safePatch } = body
    const updated: ClaudeKey = {
      ...current,
      ...safePatch,
      name: body.name?.trim() ?? current.name,
      model: body.model?.trim() ?? current.model,
      enabled,
      masked_key: maskedKey,
      fingerprint,
      status: enabled
        ? body.api_key
          ? 'unknown'
          : current.status === 'disabled'
            ? 'unknown'
            : current.status
        : 'disabled',
      ...(body.api_key
        ? { consecutive_failures: 0, last_error_code: null, cooldown_until: null }
        : {}),
      updated_at: nowIso(),
    }
    claudeKeysStore[index] = updated
    return json(request, updated)
  }),
  http.delete('*/api/v1/system/claude-keys/:keyId', ({ request, params }) => {
    const index = claudeKeysStore.findIndex((key) => key.id === params.keyId)
    if (index < 0) return notFound(request, 'Claude Key 不存在或已删除。')
    claudeKeysStore.splice(index, 1)
    return new HttpResponse(null, { status: 204, headers: echoRequestId(request) })
  }),
  http.post('*/api/v1/system/claude-keys/:keyId/test', ({ request, params }) => {
    if (!claudeKeysStore.some((key) => key.id === params.keyId)) {
      return notFound(request, 'Claude Key 不存在或已删除。')
    }
    const job = newConnectivityJob()
    const fingerprint = `claude-key-test:${String(params.keyId)}`
    const replay = replayIdempotent<AsyncAccepted>(request, fingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 202 })
    createdJobs.unshift(job)
    const accepted: AsyncAccepted = { job }
    rememberIdempotent(request, fingerprint, accepted)
    return json(request, accepted, { status: 202 })
  }),
  http.post('*/api/v1/system/claude-keys/:keyId/claude-code-settings', ({ request, params }) => {
    if (!claudeKeysStore.some((key) => key.id === params.keyId)) {
      return notFound(request, 'Claude Key 不存在或已删除。')
    }
    return json(request, { status: 'saved', target: '~/.claude/settings.json' })
  }),
  http.post('*/api/v1/system/claude-tests', ({ request }) => {
    const job = newConnectivityJob()
    const fingerprint = 'claude-gateway-test'
    const replay = replayIdempotent<AsyncAccepted>(request, fingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 202 })
    createdJobs.unshift(job)
    const accepted: AsyncAccepted = { job }
    rememberIdempotent(request, fingerprint, accepted)
    return json(request, accepted, { status: 202 })
  }),

  // 课程
  http.get('*/api/v1/courses', ({ request }) => {
    const url = new URL(request.url)
    const archived = url.searchParams.get('archived')
    const q = url.searchParams.get('q')?.toLowerCase()
    let items = baseCourses().map(withLessonCount)
    if (archived === 'true') items = items.filter((course) => course.archived)
    if (archived === 'false') items = items.filter((course) => !course.archived)
    if (q) items = items.filter((course) => course.title.toLowerCase().includes(q))
    return json(request, paginate(items, url))
  }),
  http.post('*/api/v1/courses', async ({ request }) => {
    const body = (await request.json().catch(() => null)) as Partial<CourseCreate> | null
    const fieldErrors = validateCourseFields(body)
    if (fieldErrors.length > 0) return validationProblem(request, fieldErrors)

    const fingerprint = JSON.stringify(body)
    const replay = replayIdempotent<Course>(request, fingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 201 })

    const now = nowIso()
    const course: Course = {
      id: globalThis.crypto.randomUUID(),
      title: (body?.title as string).trim(),
      description: typeof body?.description === 'string' ? body.description : '',
      archived: false,
      lesson_count: 0,
      version: 1,
      created_at: now,
      updated_at: now,
    }
    createdCourses.unshift(course)
    rememberIdempotent(request, fingerprint, course)
    return json(request, course, { status: 201 })
  }),
  http.get('*/api/v1/courses/:courseId', ({ request, params }) => {
    const course = findCourse(params.courseId as string)
    return course ? json(request, course) : notFound(request, '课程不存在或已删除。')
  }),
  http.patch('*/api/v1/courses/:courseId', async ({ request, params }) => {
    const course = findCourse(params.courseId as string)
    if (!course) return notFound(request, '课程不存在或已删除。')
    const body = (await request.json().catch(() => null)) as CourseUpdate | null
    if (!body || body.version !== course.version) {
      return problem(request, {
        status: 409,
        code: 'version_conflict',
        title: '版本冲突',
        detail: '课程已被其他操作修改，请刷新后重试。',
        extensions: { current_version: course.version },
      })
    }
    const fieldErrors: FieldError[] = []
    if (body.title !== undefined && (body.title.trim().length === 0 || body.title.length > 200)) {
      fieldErrors.push({ field: 'title', code: 'invalid', message: '课程标题为 1–200 字。' })
    }
    if (typeof body.description === 'string' && body.description.length > 5000) {
      fieldErrors.push({
        field: 'description',
        code: 'too_long',
        message: '课程描述最长 5000 字。',
      })
    }
    if (fieldErrors.length > 0) return validationProblem(request, fieldErrors)

    patchCourse(course.id, {
      ...(body.title !== undefined ? { title: body.title.trim() } : {}),
      ...(body.description !== undefined ? { description: body.description } : {}),
      ...(body.archived !== undefined ? { archived: body.archived } : {}),
      version: course.version + 1,
    })
    return json(request, findCourse(course.id))
  }),
  http.delete('*/api/v1/courses/:courseId', ({ request, params }) => {
    const courseId = params.courseId as string
    if (!findCourse(courseId)) return notFound(request, '课程不存在或已删除。')
    deletedCourseIds.add(courseId)
    for (const lesson of baseLessons()) {
      if (lesson.course_id === courseId) deletedLessonIds.add(lesson.id)
    }
    return new HttpResponse(null, { status: 204, headers: echoRequestId(request) })
  }),

  // 课时
  http.get('*/api/v1/courses/:courseId/lessons', ({ request, params }) => {
    const url = new URL(request.url)
    const items = baseLessons()
      .filter((lesson) => lesson.course_id === params.courseId)
      .sort((a, b) => a.ordinal - b.ordinal)
    return json(request, paginate(items, url))
  }),
  http.post('*/api/v1/courses/:courseId/lessons', async ({ request, params }) => {
    const course = baseCourses().find((item) => item.id === params.courseId)
    if (!course) return notFound(request, '课程不存在或已删除。')
    const body = (await request.json().catch(() => null)) as Partial<LessonCreate> | null
    const fieldErrors = validateLessonFields(body)
    if (fieldErrors.length > 0) return validationProblem(request, fieldErrors)

    const fingerprint = JSON.stringify(body)
    const replay = replayIdempotent<Lesson>(request, fingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 201 })

    const now = nowIso()
    const lesson: Lesson = {
      id: globalThis.crypto.randomUUID(),
      course_id: course.id,
      ordinal: body?.ordinal as number,
      title: (body?.title as string).trim(),
      description: typeof body?.description === 'string' ? body.description : '',
      status: 'draft',
      active_job_id: null,
      documents: { raw_text: false, student_notes: false, teacher_outline: false },
      mastery: null,
      version: 1,
      created_at: now,
      updated_at: now,
    }
    createdLessons.unshift(lesson)
    rememberIdempotent(request, fingerprint, lesson)
    return json(request, lesson, { status: 201 })
  }),
  http.get('*/api/v1/lessons/:lessonId', ({ request, params }) => {
    const lesson = findLesson(params.lessonId as string)
    return lesson ? json(request, lesson) : notFound(request, '课时不存在或已删除。')
  }),
  http.patch('*/api/v1/lessons/:lessonId', async ({ request, params }) => {
    const lesson = findLesson(params.lessonId as string)
    if (!lesson) return notFound(request, '课时不存在或已删除。')
    const body = (await request.json().catch(() => null)) as LessonUpdate | null
    if (!body || body.version !== lesson.version) {
      return problem(request, {
        status: 409,
        code: 'version_conflict',
        title: '版本冲突',
        detail: '课时已被其他操作修改，请刷新后重试。',
        extensions: { current_version: lesson.version },
      })
    }
    const fieldErrors: FieldError[] = []
    if (body.title !== undefined && (body.title.trim().length === 0 || body.title.length > 200)) {
      fieldErrors.push({ field: 'title', code: 'invalid', message: '课时标题为 1–200 字。' })
    }
    if (body.ordinal !== undefined && (!Number.isInteger(body.ordinal) || body.ordinal < 1)) {
      fieldErrors.push({ field: 'ordinal', code: 'invalid', message: '课次必须是 ≥1 的整数。' })
    }
    if (typeof body.description === 'string' && body.description.length > 5000) {
      fieldErrors.push({
        field: 'description',
        code: 'too_long',
        message: '课时描述最长 5000 字。',
      })
    }
    if (fieldErrors.length > 0) return validationProblem(request, fieldErrors)

    patchLesson(lesson.id, {
      ...(body.title !== undefined ? { title: body.title.trim() } : {}),
      ...(body.ordinal !== undefined ? { ordinal: body.ordinal } : {}),
      ...(body.description !== undefined ? { description: body.description } : {}),
      version: lesson.version + 1,
    })
    return json(request, findLesson(lesson.id))
  }),
  http.delete('*/api/v1/lessons/:lessonId', ({ request, params }) => {
    const lessonId = params.lessonId as string
    if (!findLesson(lessonId)) return notFound(request, '课时不存在或已删除。')
    deletedLessonIds.add(lessonId)
    return new HttpResponse(null, { status: 204, headers: echoRequestId(request) })
  }),

  // 素材导入（文本 / URL / 上传）：统一创建 lesson_import 任务
  http.post('*/api/v1/lessons/:lessonId/materials/text', async ({ request, params }) => {
    const lesson = findLesson(params.lessonId as string)
    if (!lesson) return notFound(request, '课时不存在或已删除。')
    const body = (await request.json().catch(() => null)) as Partial<TextMaterialCreate> | null
    const fieldErrors: FieldError[] = []
    if (!body || typeof body.text !== 'string' || body.text.trim().length === 0) {
      fieldErrors.push({ field: 'text', code: 'required', message: '请输入要导入的文本。' })
    }
    if (typeof body?.title === 'string' && body.title.length > 200) {
      fieldErrors.push({ field: 'title', code: 'too_long', message: '素材标题最长 200 字。' })
    }
    if (fieldErrors.length > 0) return validationProblem(request, fieldErrors)

    const material: Material = {
      id: globalThis.crypto.randomUUID(),
      lesson_id: lesson.id,
      kind: 'text',
      status: 'pending',
      source_url: null,
      original_name: typeof body?.title === 'string' && body.title ? body.title : '文本素材',
      created_at: nowIso(),
    }
    return acceptMaterialImport(request, lesson, material, JSON.stringify(body))
  }),
  http.post('*/api/v1/lessons/:lessonId/materials/url', async ({ request, params }) => {
    const lesson = findLesson(params.lessonId as string)
    if (!lesson) return notFound(request, '课时不存在或已删除。')
    const body = (await request.json().catch(() => null)) as Partial<UrlMaterialCreate> | null
    const fieldErrors: FieldError[] = []
    if (!body || typeof body.url !== 'string' || !/^https:\/\//.test(body.url)) {
      fieldErrors.push({
        field: 'url',
        code: 'invalid',
        message: '请输入 https:// 开头的链接。',
      })
    }
    if (typeof body?.preferred_language === 'string' && body.preferred_language.length > 30) {
      fieldErrors.push({
        field: 'preferred_language',
        code: 'too_long',
        message: '语言偏好最长 30 字。',
      })
    }
    if (fieldErrors.length > 0) return validationProblem(request, fieldErrors)

    const material: Material = {
      id: globalThis.crypto.randomUUID(),
      lesson_id: lesson.id,
      kind: 'url',
      status: 'pending',
      source_url: body?.url as string,
      original_name: null,
      created_at: nowIso(),
    }
    return acceptMaterialImport(request, lesson, material, JSON.stringify(body))
  }),
  http.post('*/api/v1/lessons/:lessonId/materials/upload', async ({ request, params }) => {
    const lesson = findLesson(params.lessonId as string)
    if (!lesson) return notFound(request, '课时不存在或已删除。')
    const form = await request.formData().catch(() => null)
    const file = form?.get('file')
    // 不用 instanceof File：MSW/undici 解析出的 File 与 jsdom File 不同源。
    if (!form || !file || typeof file === 'string' || file.size === 0) {
      return validationProblem(request, [
        { field: 'file', code: 'required', message: '请选择要上传的文件。' },
      ])
    }
    if (file.size > MAX_UPLOAD_BYTES) {
      return problem(request, {
        status: 413,
        code: 'upload_too_large',
        title: '文件超过大小限制',
        detail: '文件超过上传大小限制，请选择更小的文件。',
      })
    }
    const material: Material = {
      id: globalThis.crypto.randomUUID(),
      lesson_id: lesson.id,
      kind: 'upload',
      status: 'pending',
      source_url: null,
      original_name: file.name,
      created_at: nowIso(),
    }
    return acceptMaterialImport(request, lesson, material, `upload:${file.name}:${file.size}`)
  }),
  http.get('*/api/v1/lessons/:lessonId/materials', ({ request, params }) => {
    const url = new URL(request.url)
    const items = createdMaterials.filter((material) => material.lesson_id === params.lessonId)
    return json(request, paginate(items, url))
  }),

  // 测验生成：创建 quiz_generate 后台任务；题目发布由 Worker 成功后完成。
  http.post('*/api/v1/lessons/:lessonId/quiz-generations', async ({ request, params }) => {
    const lesson = findLesson(params.lessonId as string)
    if (!lesson) return notFound(request, '课时不存在或已删除。')
    const body = (await request.json().catch(() => ({}))) as Partial<QuizGenerationCreate> | null
    const fieldErrors: FieldError[] = []
    if (
      body?.question_count !== undefined &&
      body.question_count !== null &&
      (!Number.isInteger(body.question_count) ||
        body.question_count < 5 ||
        body.question_count > 15)
    ) {
      fieldErrors.push({
        field: 'question_count',
        code: 'invalid',
        message: '题量必须是 5–15，或留空由策略决定。',
      })
    }
    if (typeof body?.title === 'string' && body.title.length > 300) {
      fieldErrors.push({ field: 'title', code: 'too_long', message: '测验标题最长 300 字。' })
    }
    if (fieldErrors.length > 0) return validationProblem(request, fieldErrors)

    const fingerprint = `quiz-generation:${lesson.id}:${JSON.stringify(body ?? {})}`
    const replay = replayIdempotent<AsyncAccepted>(request, fingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 202 })

    if (lesson.status === 'processing' && lesson.active_job_id) {
      return problem(request, {
        status: 409,
        code: 'resource_busy',
        title: '已有任务在进行',
        detail: '同一课时同时只允许一个生成任务，请等待现有任务完成。',
        extensions: { active_job_id: lesson.active_job_id, generation_family: 'lesson_generation' },
      })
    }

    const job = newQuizGenerationJob(lesson.id)
    createdJobs.unshift(job)
    patchLesson(lesson.id, { status: 'processing', active_job_id: job.id })
    const accepted: AsyncAccepted = { job }
    rememberIdempotent(request, fingerprint, accepted)
    return json(request, accepted, { status: 202 })
  }),

  // 总结评估：创建 summary_evaluate 后台任务；评估结果由 Worker 完成后写回。
  http.post('*/api/v1/lessons/:lessonId/summary-evaluations', async ({ request, params }) => {
    const lesson = findLesson(params.lessonId as string)
    if (!lesson) return notFound(request, '课时不存在或已删除。')
    const body = (await request
      .json()
      .catch(() => null)) as Partial<CreateSummaryEvaluationBody> | null
    const fieldErrors: FieldError[] = []
    if (typeof body?.summary !== 'string' || body.summary.trim().length === 0) {
      fieldErrors.push({ field: 'summary', code: 'required', message: '总结内容为 1–50000 字。' })
    } else if (body.summary.length > 50_000) {
      fieldErrors.push({ field: 'summary', code: 'too_long', message: '总结内容为 1–50000 字。' })
    }
    if (fieldErrors.length > 0) return validationProblem(request, fieldErrors)

    const fingerprint = `summary-evaluation:${lesson.id}:${JSON.stringify(body ?? {})}`
    const replay = replayIdempotent<SummaryEvaluationAccepted>(request, fingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 202 })

    const evaluation: SummaryEvaluation = {
      id: globalThis.crypto.randomUUID(),
      lesson_id: lesson.id,
      status: 'evaluating',
      coverage_score: null,
      correct_points: [],
      misconceptions: [],
      missing_points: [],
      suggestions: [],
      error: null,
      created_at: nowIso(),
      completed_at: null,
    }
    const job = newSummaryEvaluationJob(evaluation.id)
    createdJobs.unshift(job)
    summaryEvaluationsStore.set(evaluation.id, evaluation)
    summaryEvaluationJobs.set(evaluation.id, job.id)
    const accepted: SummaryEvaluationAccepted = { evaluation, job }
    rememberIdempotent(request, fingerprint, accepted)
    return json(request, accepted, { status: 202 })
  }),
  http.get('*/api/v1/summary-evaluations/:evaluationId', ({ request, params }) => {
    const evaluation = summaryEvaluationsStore.get(params.evaluationId as string)
    if (!evaluation) return notFound(request, '评估不存在或已删除。')
    // 模拟 Worker：evaluating 状态第 2 次查询时完成（同 attempt 批阅推进）。
    if (evaluation.status === 'evaluating') {
      const count = (summaryEvaluationGetCounts.get(evaluation.id) ?? 0) + 1
      summaryEvaluationGetCounts.set(evaluation.id, count)
      if (count >= 2) advanceSummaryEvaluation(evaluation)
    }
    return json(request, evaluation)
  }),

  // 任务
  http.get('*/api/v1/jobs', ({ request }) => {
    const url = new URL(request.url)
    let items = baseJobs()
    if (url.searchParams.get('active') === 'true') {
      items = items.filter((job) => isJobActive(job.status))
    }
    const status = url.searchParams.get('status')
    if (status) items = items.filter((job) => job.status === status)
    const kind = url.searchParams.get('kind')
    if (kind) items = items.filter((job) => job.kind === kind)
    const aggregateId = url.searchParams.get('aggregate_id')
    if (aggregateId) items = items.filter((job) => job.aggregate_id === aggregateId)
    return json(request, paginate(items, url))
  }),
  // 全局任务事件流：常驻 SSE（fetch 传输可拦截）；推送由控制器注入。
  // 必须先于 /jobs/:jobId 声明，否则 'stream' 会被当作 jobId 匹配。
  jobStreamMock.handler,
  http.get('*/api/v1/jobs/:jobId', ({ request, params }) => {
    const job = findJob(params.jobId as string)
    return job ? json(request, job) : notFound(request, '任务不存在。')
  }),
  http.get('*/api/v1/jobs/:jobId/events', ({ request, params }) => {
    const job = findJob(params.jobId as string)
    if (!job) return notFound(request, '任务不存在。')
    const url = new URL(request.url)
    return json(request, paginate(demoJobEvents(job.id), url))
  }),
  http.post('*/api/v1/jobs/:jobId/retry', ({ request, params }) => {
    const job = findJob(params.jobId as string)
    if (!job) return notFound(request, '任务不存在。')
    if (!request.headers.get('Idempotency-Key')) {
      return problem(request, {
        status: 400,
        code: 'bad_request',
        title: '请求无效',
        detail: '缺少 Idempotency-Key 请求头。',
      })
    }

    const fingerprint = `retry:${job.id}`
    const replay = replayIdempotent<{ job: Job }>(request, fingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 202 })

    if (!(job.status === 'failed' && job.retryable)) {
      return problem(request, {
        status: 409,
        code: 'job_not_retryable',
        title: '任务不可重试',
        detail:
          job.status === 'failed'
            ? '该任务的失败类型不支持重试，请按任务详情中的建议操作。'
            : '只有失败且可重试的任务才能重试。',
        extensions: { status: job.status, retryable: job.retryable },
      })
    }
    const now = nowIso()
    const retried: Job = {
      ...job,
      id: globalThis.crypto.randomUUID(),
      status: 'queued',
      stage: '排队中',
      progress: 0,
      message: null,
      attempt_count: 1,
      retryable: false,
      cancel_requested: false,
      error: null,
      retry_of_job_id: job.id,
      created_at: now,
      started_at: null,
      completed_at: null,
      updated_at: now,
    }
    createdJobs.unshift(retried)
    // 批阅任务重试：对应 Attempt 回到 grading，由 advanceGrading 模拟 Worker 完成
    if (job.kind === 'attempt_grade' && job.aggregate_id) {
      const attempt = attemptsStore.get(job.aggregate_id)
      if (attempt && attempt.status === 'grading_failed') {
        attempt.status = 'grading'
        attempt.grading_job_id = retried.id
        attemptGetCounts.set(attempt.id, 0)
      }
    }
    rememberIdempotent(request, fingerprint, { job: retried })
    return json(request, { job: retried }, { status: 202 })
  }),
  http.post('*/api/v1/jobs/:jobId/cancel', ({ request, params }) => {
    const job = findJob(params.jobId as string)
    if (!job) return notFound(request, '任务不存在。')
    if (!request.headers.get('Idempotency-Key')) {
      return problem(request, {
        status: 400,
        code: 'bad_request',
        title: '请求无效',
        detail: '缺少 Idempotency-Key 请求头。',
      })
    }
    const fingerprint = `cancel:${job.id}`
    const replay = replayIdempotent<Job>(request, fingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 202 })
    if (!isJobActive(job.status)) {
      return problem(request, {
        status: 422,
        code: 'validation_error',
        title: '无法取消',
        detail: '只有进行中的任务可以取消。',
        extensions: { status: job.status },
      })
    }
    patchJob(job.id, {
      status: 'cancelled',
      cancel_requested: true,
      message: '任务已取消。',
      completed_at: nowIso(),
    })
    const cancelled = findJob(job.id) as Job
    rememberIdempotent(request, fingerprint, cancelled)
    return json(request, cancelled, { status: 202 })
  }),

  // 课程进度
  http.get('*/api/v1/courses/:courseId/progress', ({ request, params }) => {
    const progress = demoProgressByCourse[params.courseId as string]
    return progress ? json(request, progress) : notFound(request, '课程进度不存在。')
  }),

  // 文档：当前版本集合 / 指定类型当前文档 / 历史版本（游标分页）/ 单版本内容
  http.get('*/api/v1/lessons/:lessonId/documents', ({ request, params }) => {
    if (!findLesson(params.lessonId as string)) return notFound(request, '课时不存在或已删除。')
    return json(request, {
      items: activeDocumentsOf(params.lessonId as string).map(toDocumentSummary),
    })
  }),
  http.get('*/api/v1/lessons/:lessonId/documents/:kind/versions', ({ request, params }) => {
    const items = demoDocuments
      .filter((doc) => doc.lesson_id === params.lessonId && doc.kind === params.kind)
      .sort((a, b) => b.revision - a.revision)
      .map(toDocumentSummary)
    return json(request, paginate(items, new URL(request.url)))
  }),
  http.get('*/api/v1/lessons/:lessonId/documents/:kind', ({ request, params }) => {
    const doc = activeDocumentsOf(params.lessonId as string).find((d) => d.kind === params.kind)
    return doc ? json(request, doc) : notFound(request, '该类型的文档尚未生成。')
  }),
  http.get('*/api/v1/documents/:documentId', ({ request, params }) => {
    const doc = demoDocuments.find((d) => d.id === params.documentId)
    return doc ? json(request, doc) : notFound(request, '文档版本不存在。')
  }),

  // 课时进度（掌握度与趋势，后端计算，前端只读）
  http.get('*/api/v1/lessons/:lessonId/progress', ({ request, params }) => {
    const progress = demoProgressByLesson[params.lessonId as string]
    return progress ? json(request, progress) : notFound(request, '课时进度不存在。')
  }),

  // 测验与作答
  http.get('*/api/v1/lessons/:lessonId/quizzes', ({ request, params }) => {
    const url = new URL(request.url)
    const items = baseQuizzes()
      .filter((quiz) => quiz.lesson_id === params.lessonId)
      .map(toQuizSummary)
    return json(request, paginate(items, url))
  }),
  // 课时作答历史（started_at 倒序；无作答时为空列表，同 quizzes mock 语义）
  http.get('*/api/v1/lessons/:lessonId/attempts', ({ request, params }) => {
    const url = new URL(request.url)
    const lessonQuizIds = new Set(
      baseQuizzes()
        .filter((quiz) => quiz.lesson_id === params.lessonId)
        .map((quiz) => quiz.id),
    )
    const items = [...attemptsStore.values()]
      .filter((attempt) => lessonQuizIds.has(attempt.quiz_id))
      .sort((a, b) => b.started_at.localeCompare(a.started_at))
      .map(toAttemptSummary)
    return json(request, paginate(items, url))
  }),
  http.get('*/api/v1/quizzes/:quizId', ({ request, params }) => {
    const quiz = findQuiz(params.quizId as string)
    return quiz ? json(request, quiz) : notFound(request, '测验不存在。')
  }),
  http.post('*/api/v1/quizzes/:quizId/attempts', ({ request, params }) => {
    const quiz = findQuiz(params.quizId as string)
    if (!quiz) return notFound(request, '测验不存在。')

    const fingerprint = `create-attempt:${quiz.id}`
    const replay = replayIdempotent<Attempt>(request, fingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 201 })

    const attempt: Attempt = {
      id: globalThis.crypto.randomUUID(),
      quiz_id: quiz.id,
      status: 'in_progress',
      version: 1,
      auto_score: null,
      final_score: null,
      max_score: quiz.questions.reduce((total, question) => total + question.score, 0),
      grading_job_id: null,
      grading_error_code: null,
      grading_error_detail: null,
      questions: structuredClone(quiz.questions),
      answers: [],
      started_at: nowIso(),
    }
    attemptsStore.set(attempt.id, attempt)
    rememberIdempotent(request, fingerprint, attempt)
    return json(request, attempt, {
      status: 201,
      headers: { Location: `/api/v1/attempts/${attempt.id}` },
    })
  }),
  http.get('*/api/v1/attempts/:attemptId', ({ request, params }) => {
    const attempt = findAttempt(params.attemptId as string)
    if (!attempt) return notFound(request, '作答不存在。')
    if (attempt.status === 'grading') {
      const count = (attemptGetCounts.get(attempt.id) ?? 0) + 1
      attemptGetCounts.set(attempt.id, count)
      if (count >= 2) advanceGrading(attempt)
    }
    return json(request, attempt)
  }),
  http.patch('*/api/v1/attempts/:attemptId/answers/:questionId', async ({ request, params }) => {
    const attempt = findAttempt(params.attemptId as string)
    if (!attempt) return notFound(request, '作答不存在。')
    if (attempt.status !== 'in_progress') {
      return problem(request, {
        status: 409,
        code: 'attempt_already_submitted',
        title: '答卷已提交',
        detail: '本次作答已经提交，不能再修改答案。',
      })
    }
    const questionId = params.questionId as string
    const question = attempt.questions.find((item) => item.id === questionId)
    if (!question) return notFound(request, '题目不存在于本次作答中。')

    const body = (await request.json().catch(() => null)) as AnswerSave | null
    if (!body || typeof body.answer_version !== 'number' || body.answer === undefined) {
      return validationProblem(request, [
        { field: 'answer_version', code: 'required', message: '请求体不符合 AnswerSave 契约。' },
      ])
    }
    const existing = attempt.answers.find((item) => item.question_id === questionId)
    const currentAnswerVersion = existing?.answer_version ?? 0
    if (body.answer_version !== currentAnswerVersion) {
      return problem(request, {
        status: 409,
        code: 'version_conflict',
        title: '答案版本冲突',
        detail: '服务端已有更新的答案。',
        extensions: {
          current_answer: existing?.answer ?? null,
          current_answer_version: currentAnswerVersion,
          current_attempt_version: attempt.version,
        },
      })
    }

    const saved: AttemptAnswer = {
      question_id: questionId,
      answer: body.answer,
      answer_version: currentAnswerVersion + 1,
      updated_at: nowIso(),
    }
    if (existing) {
      attempt.answers = attempt.answers.map((item) =>
        item.question_id === questionId ? saved : item,
      )
    } else {
      attempt.answers = [...attempt.answers, saved]
    }
    attempt.version += 1
    const response: SavedAnswer = {
      attempt_id: attempt.id,
      question_id: questionId,
      answer: body.answer,
      answer_version: saved.answer_version,
      attempt_version: attempt.version,
      saved_at: nowIso(),
    }
    return json(request, response)
  }),
  http.post('*/api/v1/attempts/:attemptId/submit', async ({ request, params }) => {
    const attempt = findAttempt(params.attemptId as string)
    if (!attempt) return notFound(request, '作答不存在。')

    const body = (await request.json().catch(() => null)) as SubmitAttemptBody | null
    const fingerprint = `submit:${attempt.id}:${JSON.stringify(body)}`
    const replay = replayIdempotent<AttemptSubmitCompleted | AttemptSubmitAccepted>(
      request,
      fingerprint,
    )
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) {
      const status = 'job' in (replay.replay as AttemptSubmitAccepted) ? 202 : 200
      return json(request, replay.replay, { status })
    }

    if (attempt.status !== 'in_progress') {
      return problem(request, {
        status: 409,
        code: 'attempt_already_submitted',
        title: '答卷已提交',
        detail: '本次作答已经提交，请查看批阅结果。',
      })
    }
    if (!body || body.version !== attempt.version) {
      return problem(request, {
        status: 409,
        code: 'version_conflict',
        title: '版本冲突',
        detail: '作答版本已变化，请刷新后重试提交。',
        extensions: { current_version: attempt.version },
      })
    }

    attempt.auto_score = gradeObjectiveAnswers(attempt)
    attempt.submitted_at = nowIso()
    attempt.version += 1
    const hasSubjective = attempt.questions.some(
      (question: Question) => !isObjectiveQuestion(question.type),
    )

    if (!hasSubjective) {
      attempt.status = 'graded'
      attempt.final_score = attempt.auto_score
      attempt.graded_at = nowIso()
      const completed: AttemptSubmitCompleted = { attempt: structuredClone(attempt) }
      rememberIdempotent(request, fingerprint, completed)
      return json(request, completed, { status: 200 })
    }

    const job = newGradingJob(attempt)
    createdJobs.unshift(job)
    attempt.status = 'grading'
    attempt.grading_job_id = job.id
    attemptGetCounts.set(attempt.id, 0)
    const accepted: AttemptSubmitAccepted = { attempt: structuredClone(attempt), job }
    rememberIdempotent(request, fingerprint, accepted)
    return json(request, accepted, { status: 202 })
  }),
  http.post('*/api/v1/attempts/:attemptId/grading-jobs', ({ request, params }) => {
    const attempt = findAttempt(params.attemptId as string)
    if (!attempt) return notFound(request, '作答不存在。')

    const fingerprint = `grading-jobs:${attempt.id}`
    const replay = replayIdempotent<AsyncAccepted>(request, fingerprint)
    if (replay && 'conflict' in replay) return idempotencyConflictProblem(request)
    if (replay) return json(request, replay.replay, { status: 202 })

    if (attempt.status === 'grading') {
      return problem(request, {
        status: 409,
        code: 'resource_busy',
        title: '已有批阅任务在进行',
        detail: '当前已存在活动的批阅任务，请等待其完成。',
        extensions: { active_job_id: attempt.grading_job_id },
      })
    }
    if (attempt.status !== 'grading_failed') {
      return problem(request, {
        status: 409,
        code: 'bad_request',
        title: '无法重新批阅',
        detail: '只有批阅失败的作答可以创建新的批阅任务。',
      })
    }

    const job = newGradingJob(attempt)
    createdJobs.unshift(job)
    attempt.status = 'grading'
    attempt.grading_error_code = null
    attempt.grading_error_detail = null
    attempt.grading_job_id = job.id
    attemptGetCounts.set(attempt.id, 0)
    const accepted: AsyncAccepted = { job }
    rememberIdempotent(request, fingerprint, accepted)
    return json(request, accepted, { status: 202 })
  }),

  // 弱点：curated fixtures + 过滤（course_id/lesson_id/mastered/severity）+ 游标分页
  http.get('*/api/v1/weaknesses', ({ request }) => {
    const url = new URL(request.url)
    let items = demoWeaknesses
    const courseId = url.searchParams.get('course_id')
    if (courseId) items = items.filter((w) => w.course_id === courseId)
    const lessonId = url.searchParams.get('lesson_id')
    if (lessonId) items = items.filter((w) => w.lesson_id === lessonId)
    const mastered = url.searchParams.get('mastered')
    if (mastered === 'true') items = items.filter((w) => w.mastered)
    if (mastered === 'false') items = items.filter((w) => !w.mastered)
    const severity = url.searchParams.get('severity')
    if (severity) items = items.filter((w) => w.severity === Number(severity))
    return json(request, paginate(items, url))
  }),
  http.get('*/api/v1/weaknesses/:weaknessId', ({ request, params }) => {
    const weakness = demoWeaknesses.find((w) => w.id === params.weaknessId)
    return weakness ? json(request, weakness) : notFound(request, '弱点不存在。')
  }),
]

const generatedHandlers = [
  ...getAttemptsMock(),
  ...getBackupsMock(),
  ...getCoursesMock(),
  ...getDocumentsMock(),
  ...getExportsMock(),
  ...getFilesMock(),
  ...getJobsMock(),
  ...getLessonsMock(),
  ...getMaterialsMock(),
  ...getProgressMock(),
  ...getQuizzesMock(),
  ...getSummariesMock(),
  ...getSystemMock(),
  ...getWeaknessesMock(),
]

// MSW 按声明顺序匹配，curated 优先于生成兜底。
export const handlers = [...curatedHandlers, ...generatedHandlers]
