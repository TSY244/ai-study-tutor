import type { QueryClient } from '@tanstack/react-query'

import { queryKeys } from '@/api/query-client'

/**
 * Mutation 成功后的精确失效关系（02-frontend-handoff.md §5.1）。
 *
 * 规则：
 * - 只失效与被改资源有因果关系的 Query Key；禁止无参 invalidateQueries()。
 * - 列表 Key 形如 ['courses', filters]，用资源前缀（['courses']）失效即可
 *   命中所有过滤变体，这是前缀匹配而非全局失效。
 * - 每个函数只声明关系；由 Mutation 的 onSuccess 调用，保证先写后失效。
 */

/** 新课程：只影响课程列表（各过滤变体）。 */
export async function invalidateCoursesList(client: QueryClient): Promise<void> {
  await client.invalidateQueries({ queryKey: ['courses'] })
}

/** 课程字段更新（标题/归档等）：列表 + 该课程详情。 */
export async function invalidateCourseChanged(
  client: QueryClient,
  courseId: string,
): Promise<void> {
  await Promise.all([
    client.invalidateQueries({ queryKey: ['courses'] }),
    client.invalidateQueries({ queryKey: queryKeys.course(courseId) }),
  ])
}

/** 课程删除：列表失效，详情直接从缓存移除（避免 404 重取）。 */
export async function invalidateCourseRemoved(
  client: QueryClient,
  courseId: string,
): Promise<void> {
  await invalidateCoursesRemoved(client, [courseId])
}

/** 批量删除课程：逐个移除详情，共享列表只刷新一次。 */
export async function invalidateCoursesRemoved(
  client: QueryClient,
  courseIds: string[],
): Promise<void> {
  for (const courseId of courseIds) {
    client.removeQueries({ queryKey: queryKeys.course(courseId) })
    client.removeQueries({ queryKey: queryKeys.lessons(courseId) })
    client.removeQueries({ queryKey: queryKeys.progress(courseId) })
  }
  await client.invalidateQueries({ queryKey: ['courses'] })
}

/**
 * 课时创建/结构变化：课时列表、所属课程详情（lesson_count）、
 * 课程列表（计数列）与课程进度。课时不影响任务/弱点等无关资源。
 */
export async function invalidateLessonCreated(
  client: QueryClient,
  courseId: string,
): Promise<void> {
  await Promise.all([
    client.invalidateQueries({ queryKey: queryKeys.lessons(courseId) }),
    client.invalidateQueries({ queryKey: queryKeys.course(courseId) }),
    client.invalidateQueries({ queryKey: queryKeys.progress(courseId) }),
    client.invalidateQueries({ queryKey: ['courses'] }),
  ])
}

/** 课时字段更新（标题/课次/描述）：课时详情 + 所属课程的课时列表。 */
export async function invalidateLessonChanged(
  client: QueryClient,
  lessonId: string,
  courseId: string,
): Promise<void> {
  await Promise.all([
    client.invalidateQueries({ queryKey: queryKeys.lesson(lessonId) }),
    client.invalidateQueries({ queryKey: queryKeys.lessons(courseId) }),
  ])
}

/** 课时删除：详情移除（避免 404 重取）+ 列表/课程/进度失效。 */
export async function invalidateLessonRemoved(
  client: QueryClient,
  lessonId: string,
  courseId: string,
): Promise<void> {
  await invalidateLessonsRemoved(client, [lessonId], courseId)
}

/** 批量删除课时：逐个移除详情，共享列表/课程/进度各刷新一次。 */
export async function invalidateLessonsRemoved(
  client: QueryClient,
  lessonIds: string[],
  courseId: string,
): Promise<void> {
  for (const lessonId of lessonIds) {
    client.removeQueries({ queryKey: queryKeys.lesson(lessonId) })
    for (const prefix of [
      'documents',
      'document',
      'document-versions',
      'materials',
      'quizzes',
      'lesson-attempts',
      'summary-evaluations',
    ]) {
      client.removeQueries({ queryKey: [prefix, lessonId] })
    }
    client.removeQueries({ queryKey: queryKeys.progressLesson(lessonId) })
  }
  await Promise.all([
    client.invalidateQueries({ queryKey: queryKeys.lessons(courseId) }),
    client.invalidateQueries({ queryKey: queryKeys.course(courseId) }),
    client.invalidateQueries({ queryKey: queryKeys.progress(courseId) }),
    client.invalidateQueries({ queryKey: ['courses'] }),
    client.invalidateQueries({ queryKey: ['knowledge-points', courseId] }),
    client.invalidateQueries({ queryKey: ['weaknesses'] }),
    client.invalidateQueries({ queryKey: ['attempt'] }),
  ])
}

/**
 * 素材导入已被接受（任务创建）：课时详情（status/active_job_id 变化）、
 * 所属课程课时列表（状态列）与任务中心；不改变课时计数，不碰无关资源。
 */
export async function invalidateMaterialImportAccepted(
  client: QueryClient,
  lessonId: string,
  courseId: string,
  jobId?: string,
): Promise<void> {
  await Promise.all([
    client.invalidateQueries({ queryKey: queryKeys.lesson(lessonId) }),
    client.invalidateQueries({ queryKey: queryKeys.lessons(courseId) }),
    jobId
      ? Promise.all([
          client.invalidateQueries({ queryKey: ['jobs'] }),
          client.invalidateQueries({ queryKey: queryKeys.job(jobId) }),
        ])
      : client.invalidateQueries({ queryKey: ['jobs'] }),
  ])
}

/**
 * 测验生成任务已接受：会占用课时 lesson_generation 活动槽，因此刷新课时、
 * 所属课时列表、测验列表和任务中心。题目实际发布仍以 Worker 成功后的
 * REST/SSE 对账为准。
 */
export async function invalidateQuizGenerationAccepted(
  client: QueryClient,
  lessonId: string,
  courseId: string,
  jobId?: string,
): Promise<void> {
  await Promise.all([
    client.invalidateQueries({ queryKey: queryKeys.lesson(lessonId) }),
    client.invalidateQueries({ queryKey: queryKeys.lessons(courseId) }),
    client.invalidateQueries({ queryKey: queryKeys.quizzes(lessonId) }),
    jobId
      ? Promise.all([
          client.invalidateQueries({ queryKey: ['jobs'] }),
          client.invalidateQueries({ queryKey: queryKeys.job(jobId) }),
        ])
      : client.invalidateQueries({ queryKey: ['jobs'] }),
  ])
}

/** 任务状态变化（F3 起使用）：任务列表（各过滤变体）+ 单个任务详情。 */
export async function invalidateJobChanged(client: QueryClient, jobId?: string): Promise<void> {
  const jobs = client.invalidateQueries({ queryKey: ['jobs'] })
  if (jobId) {
    await Promise.all([jobs, client.invalidateQueries({ queryKey: queryKeys.job(jobId) })])
  } else {
    await jobs
  }
}

/**
 * 总结评估任务已接受：评估查询由 SSE 终态对账驱动刷新；
 * 这里只失效任务中心（新任务出现在列表/角标）。
 */
export async function invalidateSummaryEvaluationAccepted(
  client: QueryClient,
  jobId?: string,
): Promise<void> {
  await invalidateJobChanged(client, jobId)
}

/**
 * 文档可能已被生成任务重写（F4）：当前文档集合、各 kind 当前文档与
 * 历史版本列表一起失效；document-version 快照内容不可变，不失效。
 */
export async function invalidateDocumentsChanged(
  client: QueryClient,
  lessonId: string,
): Promise<void> {
  await Promise.all([
    client.invalidateQueries({ queryKey: queryKeys.documents(lessonId) }),
    client.invalidateQueries({ queryKey: ['document', lessonId] }),
    client.invalidateQueries({ queryKey: ['document-versions', lessonId] }),
  ])
}
