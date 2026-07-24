import { QueryClient } from '@tanstack/react-query'
import { describe, expect, it, vi } from 'vitest'

import {
  invalidateCourseChanged,
  invalidateCourseRemoved,
  invalidateCoursesList,
  invalidateJobChanged,
  invalidateLessonCreated,
  invalidateQuizGenerationAccepted,
} from '@/api/invalidation'
import { queryKeys } from '@/api/query-client'

/**
 * 失效关系测试：用 isInvalidated 标记断言“精确失效”——
 * 相关资源被标记失效，无关资源保持有效；同时用间谍证明
 * 从未发生无参全局失效。
 */

const ALL_KEYS = [
  ['courses', { limit: 20 }],
  ['courses', { archived: true }],
  queryKeys.course('c1'),
  queryKeys.lessons('c1'),
  queryKeys.lesson('l1'),
  queryKeys.documents('l1'),
  queryKeys.quizzes('l1'),
  queryKeys.quiz('q1'),
  queryKeys.attempt('a1'),
  ['jobs', { active: true }],
  queryKeys.job('j1'),
  ['weaknesses', {}],
  queryKeys.progress('c1'),
  queryKeys.systemStatus(),
] as const

function seedClient() {
  const client = new QueryClient()
  for (const key of ALL_KEYS) client.setQueryData(key, { seeded: true })
  const spy = vi.spyOn(client, 'invalidateQueries')
  return { client, spy }
}

function invalidated(client: QueryClient, key: readonly unknown[]): boolean {
  return client.getQueryState(key)?.isInvalidated === true
}

describe('精确失效关系', () => {
  it('invalidateCoursesList：只命中课程列表的所有过滤变体', async () => {
    const { client } = seedClient()
    await invalidateCoursesList(client)

    expect(invalidated(client, ['courses', { limit: 20 }])).toBe(true)
    expect(invalidated(client, ['courses', { archived: true }])).toBe(true)
    expect(invalidated(client, queryKeys.course('c1'))).toBe(false)
    expect(invalidated(client, ['jobs', { active: true }])).toBe(false)
    expect(invalidated(client, queryKeys.progress('c1'))).toBe(false)
  })

  it('invalidateCourseChanged：列表 + 该课程详情，不碰其他资源', async () => {
    const { client } = seedClient()
    await invalidateCourseChanged(client, 'c1')

    expect(invalidated(client, ['courses', { limit: 20 }])).toBe(true)
    expect(invalidated(client, queryKeys.course('c1'))).toBe(true)
    expect(invalidated(client, queryKeys.lessons('c1'))).toBe(false)
    expect(invalidated(client, queryKeys.quiz('q1'))).toBe(false)
    expect(invalidated(client, queryKeys.systemStatus())).toBe(false)
  })

  it('invalidateCourseRemoved：详情被移除而非失效，列表失效', async () => {
    const { client } = seedClient()
    await invalidateCourseRemoved(client, 'c1')

    expect(client.getQueryState(queryKeys.course('c1'))).toBeUndefined()
    expect(client.getQueryState(queryKeys.lessons('c1'))).toBeUndefined()
    expect(client.getQueryState(queryKeys.progress('c1'))).toBeUndefined()
    expect(invalidated(client, ['courses', { limit: 20 }])).toBe(true)
    expect(invalidated(client, ['jobs', { active: true }])).toBe(false)
  })

  it('invalidateLessonCreated：课时列表/课程/进度/课程列表，不碰任务', async () => {
    const { client } = seedClient()
    await invalidateLessonCreated(client, 'c1')

    expect(invalidated(client, queryKeys.lessons('c1'))).toBe(true)
    expect(invalidated(client, queryKeys.course('c1'))).toBe(true)
    expect(invalidated(client, queryKeys.progress('c1'))).toBe(true)
    expect(invalidated(client, ['courses', { limit: 20 }])).toBe(true)
    expect(invalidated(client, ['jobs', { active: true }])).toBe(false)
    expect(invalidated(client, queryKeys.lesson('l1'))).toBe(false)
  })

  it('invalidateJobChanged：任务列表 + 指定任务详情', async () => {
    const { client } = seedClient()
    await invalidateJobChanged(client, 'j1')

    expect(invalidated(client, ['jobs', { active: true }])).toBe(true)
    expect(invalidated(client, queryKeys.job('j1'))).toBe(true)
    expect(invalidated(client, ['courses', { limit: 20 }])).toBe(false)
    expect(invalidated(client, ['weaknesses', {}])).toBe(false)
  })

  it('invalidateQuizGenerationAccepted：课时/测验/任务相关资源失效，不碰无关实体', async () => {
    const { client } = seedClient()
    await invalidateQuizGenerationAccepted(client, 'l1', 'c1', 'j1')

    expect(invalidated(client, queryKeys.lesson('l1'))).toBe(true)
    expect(invalidated(client, queryKeys.lessons('c1'))).toBe(true)
    expect(invalidated(client, queryKeys.quizzes('l1'))).toBe(true)
    expect(invalidated(client, ['jobs', { active: true }])).toBe(true)
    expect(invalidated(client, queryKeys.job('j1'))).toBe(true)
    expect(invalidated(client, ['courses', { limit: 20 }])).toBe(false)
    expect(invalidated(client, queryKeys.attempt('a1'))).toBe(false)
  })

  it('所有失效调用都带 Key——从未发生无参全局失效', async () => {
    const { client, spy } = seedClient()
    await invalidateCoursesList(client)
    await invalidateCourseChanged(client, 'c1')
    await invalidateCourseRemoved(client, 'c1')
    await invalidateLessonCreated(client, 'c1')
    await invalidateQuizGenerationAccepted(client, 'l1', 'c1', 'j1')
    await invalidateJobChanged(client, 'j1')
    await invalidateJobChanged(client)

    expect(spy).toHaveBeenCalled()
    for (const call of spy.mock.calls) {
      const filters = call[0]
      expect(filters).toBeDefined()
      expect(Array.isArray(filters?.queryKey)).toBe(true)
      expect(filters?.queryKey?.length).toBeGreaterThan(0)
    }
  })
})
