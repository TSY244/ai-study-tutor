import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { Wand2 } from 'lucide-react'
import { useState } from 'react'

import { createQuizGeneration } from '@/api/generated/endpoints/quizzes/quizzes'
import { invalidateQuizGenerationAccepted } from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import { getResourceBusyInfo, presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { FormAlert } from '@/components/ui/form-alert'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

interface GenerateQuizButtonProps {
  courseId: string
  lessonId: string
  disabled?: boolean
}

/** 创建常规测验生成 Job；题量/策略由后端读取课时上下文决定。 */
export function GenerateQuizButton({ courseId, lessonId, disabled }: GenerateQuizButtonProps) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [error, setError] = useState<unknown>(null)
  const scope = `quiz-generation:${lessonId}`
  const busy = getResourceBusyInfo(error)

  const generateMutation = useMutation({
    mutationFn: () =>
      createQuizGeneration(
        lessonId,
        { question_count: null, title: null },
        { headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) } },
      ),
    onSuccess: async (accepted) => {
      resolveIdempotencyIntent(scope)
      await invalidateQuizGenerationAccepted(queryClient, lessonId, courseId, accepted.job.id)
      await navigate({ to: '/jobs/$jobId', params: { jobId: accepted.job.id } })
    },
    onError: (mutationError) => {
      if (isApiProblemError(mutationError) && mutationError.code === 'idempotency_conflict') {
        rotateIdempotencyIntent(scope)
      }
      setError(mutationError)
    },
  })

  const presentation = error ? presentProblem(error) : null

  return (
    <div className="space-y-2">
      <Button
        size="sm"
        disabled={disabled || generateMutation.isPending}
        onClick={() => {
          setError(null)
          generateMutation.mutate()
        }}
      >
        <Wand2 aria-hidden="true" className="h-4 w-4" />
        {generateMutation.isPending ? '创建任务中…' : '生成测验'}
      </Button>
      {presentation ? (
        <div className="space-y-2">
          <FormAlert
            title={presentation.title}
            description={presentation.message}
            code={isApiProblemError(error) ? error.code : undefined}
            requestId={isApiProblemError(error) ? error.requestId : undefined}
          />
          {busy?.activeJobId ? (
            <p className="text-sm">
              <Link
                to="/jobs/$jobId"
                params={{ jobId: busy.activeJobId }}
                className="rounded-sm font-medium text-accent underline underline-offset-2"
              >
                查看进行中的生成任务
              </Link>
              <span className="ml-2 text-text-subtle">
                （{busy.generationFamily ?? 'lesson_generation'}）
              </span>
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
