import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import { createAttempt } from '@/api/generated/endpoints/attempts/attempts'
import type { QuizSummary } from '@/api/generated/model'
import { isApiProblemError } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { queryKeys } from '@/api/query-client'
import { Button } from '@/components/ui/button'
import { FormAlert } from '@/components/ui/form-alert'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

/**
 * 开始一次作答：POST /quizzes/{id}/attempts（幂等键 scope 按测验计），
 * 成功即写入 attempt 缓存并跳转专注模式。连点/网络重试复用同一 Key，
 * 不会产生重复 Attempt。
 */
export function StartAttemptButton({ quiz }: { quiz: QuizSummary }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [error, setError] = useState<unknown>(null)
  const scope = `attempt-start:${quiz.id}`

  const startMutation = useMutation({
    mutationFn: () =>
      createAttempt(quiz.id, {
        headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) },
      }),
    onSuccess: async (attempt) => {
      resolveIdempotencyIntent(scope)
      queryClient.setQueryData(queryKeys.attempt(attempt.id), attempt)
      await navigate({ to: '/attempts/$attemptId', params: { attemptId: attempt.id } })
    },
    onError: (mutationError) => {
      if (isApiProblemError(mutationError) && mutationError.code === 'idempotency_conflict') {
        rotateIdempotencyIntent(scope)
      }
      setError(mutationError)
    },
  })

  return (
    <div className="space-y-2">
      <Button
        variant="primary"
        size="sm"
        disabled={startMutation.isPending}
        onClick={() => {
          setError(null)
          startMutation.mutate()
        }}
      >
        {startMutation.isPending ? '创建中…' : '开始作答'}
      </Button>
      {error ? (
        <FormAlert
          title={presentProblem(error).title}
          description={presentProblem(error).message}
          code={isApiProblemError(error) ? error.code : undefined}
          requestId={isApiProblemError(error) ? error.requestId : undefined}
        />
      ) : null}
    </div>
  )
}
