import { beforeEach, describe, expect, it } from 'vitest'

import {
  loadRecentLearning,
  recordRecentLearning,
  RECENT_LEARNING_LIMIT,
  RECENT_LEARNING_STORAGE_KEY,
} from '@/features/learning/recent-learning'

describe('最近学习记录', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('无记录时返回空数组', () => {
    expect(loadRecentLearning()).toEqual([])
  })

  it('新记录排在最前，并带去重（同一课时只保留最新一条）', () => {
    recordRecentLearning({ courseId: 'c1', lessonId: 'l1', lessonTitle: '第一课' })
    recordRecentLearning({ courseId: 'c1', lessonId: 'l2', lessonTitle: '第二课' })
    recordRecentLearning({ courseId: 'c1', lessonId: 'l1', lessonTitle: '第一课' })

    const entries = loadRecentLearning()
    expect(entries.map((entry) => entry.lessonId)).toEqual(['l1', 'l2'])
    expect(entries[0]?.visitedAt).toBeTruthy()
  })

  it(`最多保留 ${RECENT_LEARNING_LIMIT} 条`, () => {
    for (let index = 0; index < RECENT_LEARNING_LIMIT + 3; index += 1) {
      recordRecentLearning({
        courseId: 'c1',
        lessonId: `l${index}`,
        lessonTitle: `第${index}课`,
      })
    }

    const entries = loadRecentLearning()
    expect(entries).toHaveLength(RECENT_LEARNING_LIMIT)
    expect(entries[0]?.lessonId).toBe(`l${RECENT_LEARNING_LIMIT + 2}`)
  })

  it('存储内容损坏时按空记录处理并清除', () => {
    window.localStorage.setItem(RECENT_LEARNING_STORAGE_KEY, 'not-json')
    expect(loadRecentLearning()).toEqual([])
    expect(window.localStorage.getItem(RECENT_LEARNING_STORAGE_KEY)).toBeNull()
  })

  it('过滤结构非法的条目', () => {
    window.localStorage.setItem(
      RECENT_LEARNING_STORAGE_KEY,
      JSON.stringify({
        version: 1,
        entries: [
          {
            courseId: 'c1',
            lessonId: 'l1',
            lessonTitle: '第一课',
            visitedAt: '2026-07-27T00:00:00Z',
          },
          { courseId: 'c1', lessonId: 42 },
        ],
      }),
    )

    const entries = loadRecentLearning()
    expect(entries).toHaveLength(1)
    expect(entries[0]?.lessonId).toBe('l1')
  })
})
