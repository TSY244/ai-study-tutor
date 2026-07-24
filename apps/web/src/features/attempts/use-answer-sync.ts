import { useEffect, useReducer, useRef, useState } from 'react'

import { saveAttemptAnswer } from '@/api/generated/endpoints/attempts/attempts'
import type { Attempt } from '@/api/generated/model'

import { AnswerSyncEngine } from './answer-sync-engine'

/**
 * AnswerSyncEngine 的 React 绑定。
 *
 * 引擎在 effect 中创建（StrictMode 双挂载安全：先建先毁，timers 不泄漏）；
 * 创建前返回 null，调用方渲染轻量骨架。引擎创建后通过 onChange 触发重渲染，
 * 组件直接读引擎方法（getValue/getStatus/...）取最新状态。
 *
 * attempt 对象会随 React Query 后台更新换引用，但引擎只按 attempt.id 创建一次，
 * 初始答案快照取自首次成功加载——之后本地状态由引擎权威维护。
 */
export function useAnswerSync(attempt: Attempt): AnswerSyncEngine | null {
  const [, rerender] = useReducer((count: number) => count + 1, 0)
  const [engine, setEngine] = useState<AnswerSyncEngine | null>(null)
  const snapshotRef = useRef(attempt)

  useEffect(() => {
    const snapshot = snapshotRef.current
    const created = new AnswerSyncEngine({
      attemptId: snapshot.id,
      questionIds: snapshot.questions.map((question) => question.id),
      initialAnswers: snapshot.answers,
      initialAttemptVersion: snapshot.version,
      saveFn: (questionId, body) => saveAttemptAnswer(snapshot.id, questionId, body),
      onChange: () => rerender(),
    })
    setEngine(created)
    return () => {
      created.destroy()
      setEngine(null)
    }
  }, [attempt.id])

  return engine
}
