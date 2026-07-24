import { beforeEach, describe, expect, it } from 'vitest'

import {
  beginIdempotencyIntent,
  createIdempotencyKey,
  IDEMPOTENCY_STORAGE_PREFIX,
  isIdempotencyKey,
  peekIdempotencyIntent,
  resetIdempotencyIntents,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

beforeEach(() => {
  resetIdempotencyIntents()
})

describe('createIdempotencyKey', () => {
  it('生成合法 UUID v4', () => {
    expect(isIdempotencyKey(createIdempotencyKey())).toBe(true)
  })

  it('两次生成不相同（一次意图一个 Key，不复用别的意图）', () => {
    expect(createIdempotencyKey()).not.toBe(createIdempotencyKey())
  })
})

describe('isIdempotencyKey', () => {
  it.each([
    ['771f7e5f-1c92-4b8c-9ec4-a74ed6ef6bea', true],
    ['not-a-uuid', false],
    ['', false],
  ])('isIdempotencyKey(%j) === %j', (input, expected) => {
    expect(isIdempotencyKey(input)).toBe(expected)
  })
})

describe('意图生命周期', () => {
  it('begin 复用同一意图的 Key（重试/双击同一 Key）', () => {
    const first = beginIdempotencyIntent('create-course')
    expect(beginIdempotencyIntent('create-course')).toBe(first)
  })

  it('不同 scope 是不同意图', () => {
    expect(beginIdempotencyIntent('create-course')).not.toBe(
      beginIdempotencyIntent('create-lesson'),
    )
  })

  it('peek 不创建 Key；resolve 后下一次 begin 是全新意图', () => {
    expect(peekIdempotencyIntent('create-course')).toBeUndefined()
    const first = beginIdempotencyIntent('create-course')
    resolveIdempotencyIntent('create-course')
    expect(peekIdempotencyIntent('create-course')).toBeUndefined()
    expect(beginIdempotencyIntent('create-course')).not.toBe(first)
  })

  it('rotate 换新 Key（载荷变化时）', () => {
    const first = beginIdempotencyIntent('create-course')
    const rotated = rotateIdempotencyIntent('create-course')
    expect(rotated).not.toBe(first)
    expect(beginIdempotencyIntent('create-course')).toBe(rotated)
  })

  it('刷新恢复：sessionStorage 中的 Key 被复用', () => {
    const recovered = createIdempotencyKey()
    sessionStorage.setItem(IDEMPOTENCY_STORAGE_PREFIX + 'create-course', recovered)
    expect(beginIdempotencyIntent('create-course')).toBe(recovered)
  })

  it('sessionStorage 中的损坏值被替换为合法新 Key', () => {
    sessionStorage.setItem(IDEMPOTENCY_STORAGE_PREFIX + 'create-course', 'corrupted')
    const key = beginIdempotencyIntent('create-course')
    expect(isIdempotencyKey(key)).toBe(true)
    expect(sessionStorage.getItem(IDEMPOTENCY_STORAGE_PREFIX + 'create-course')).toBe(key)
  })

  it('resetIdempotencyIntents 清空内存与持久化状态', () => {
    beginIdempotencyIntent('create-course')
    resetIdempotencyIntents()
    expect(peekIdempotencyIntent('create-course')).toBeUndefined()
    expect(sessionStorage.getItem(IDEMPOTENCY_STORAGE_PREFIX + 'create-course')).toBeNull()
  })
})
