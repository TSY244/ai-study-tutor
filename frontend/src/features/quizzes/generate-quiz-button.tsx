import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { Wand2 } from 'lucide-react'
import { useRef, useState } from 'react'

import {
  createQuizGeneration,
  createReviewQuizGeneration,
} from '@/api/generated/endpoints/quizzes/quizzes'
import { invalidateQuizGenerationAccepted } from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import { getResourceBusyInfo, presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { FormAlert } from '@/components/ui/form-alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
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

export function GenerateQuizButton({ courseId, lessonId, disabled }: GenerateQuizButtonProps) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [mode, setMode] = useState<'regular' | 'review'>('regular')
  const [count, setCount] = useState('')
  const [notes, setNotes] = useState('')
  const [validation, setValidation] = useState<string | null>(null)
  const [error, setError] = useState<unknown>(null)
  const lastPayload = useRef<string | null>(null)
  const scope = `quiz-generation:${lessonId}:${mode}`
  const busy = getResourceBusyInfo(error)
  const generation = useMutation({
    mutationFn: () => {
      const payload = {
        question_count: count ? Number(count) : null,
        generation_notes: notes.trim(),
      }
      const signature = JSON.stringify({ mode, ...payload })
      if (lastPayload.current && lastPayload.current !== signature) rotateIdempotencyIntent(scope)
      lastPayload.current = signature
      const options = { headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) } }
      return mode === 'review'
        ? createReviewQuizGeneration(
            lessonId,
            { ...payload, include_lesson_ids: [], weakness_ids: [] },
            options,
          )
        : createQuizGeneration(lessonId, { ...payload, title: null }, options)
    },
    onSuccess: async (accepted) => {
      resolveIdempotencyIntent(scope)
      setOpen(false)
      await invalidateQuizGenerationAccepted(queryClient, lessonId, courseId, accepted.job.id)
      await navigate({ to: '/jobs/$jobId', params: { jobId: accepted.job.id } })
    },
    onError: (err) => {
      if (isApiProblemError(err) && err.code === 'idempotency_conflict')
        rotateIdempotencyIntent(scope)
      setError(err)
    },
  })
  const presentation = error ? presentProblem(error) : null
  return (
    <>
      <Button
        size="sm"
        disabled={disabled || generation.isPending}
        onClick={() => {
          setError(null)
          setValidation(null)
          setOpen(true)
        }}
      >
        <Wand2 aria-hidden="true" className="h-4 w-4" />
        生成测验
      </Button>
      <Dialog
        open={open}
        onOpenChange={(value) => {
          if (!generation.isPending) setOpen(value)
        }}
      >
        <DialogContent
          title="生成知识点测验"
          description="AI 会先分析并保存课程知识点，再围绕知识点出题。你可以指定本次练习的重点和题型偏好。"
        >
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault()
              setError(null)
              setValidation(null)
              if (
                count &&
                (!Number.isInteger(Number(count)) || Number(count) < 5 || Number(count) > 15)
              ) {
                setValidation('题量须为 5 至 15 的整数。')
                return
              }
              generation.mutate()
            }}
          >
            <div className="space-y-1.5">
              <Label htmlFor="quiz-generation-mode">测验类型</Label>
              <select
                id="quiz-generation-mode"
                value={mode}
                disabled={generation.isPending}
                onChange={(e) => setMode(e.target.value as 'regular' | 'review')}
                className="w-full rounded-md border border-border bg-surface p-2 text-sm text-text"
              >
                <option value="regular">常规测验</option>
                <option value="review">复习测验</option>
              </select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="quiz-generation-count">题量（可选）</Label>
              <Input
                id="quiz-generation-count"
                type="number"
                min={5}
                max={15}
                step={1}
                placeholder="留空按课时策略生成"
                value={count}
                disabled={generation.isPending}
                onChange={(e) => setCount(e.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="quiz-generation-notes">出题备注（可选）</Label>
              <Textarea
                id="quiz-generation-notes"
                rows={5}
                maxLength={3000}
                value={notes}
                disabled={generation.isPending}
                onChange={(e) => setNotes(e.target.value)}
                placeholder="例如：重点覆盖边界条件和常见误区，多出应用题、案例分析题，少出纯记忆题；难度中等。"
                aria-describedby="quiz-generation-notes-hint"
              />
              <p id="quiz-generation-notes-hint" className="text-xs text-text-muted">
                可写知识点范围、题型比例和难度偏好；题目仍须有资料依据。{notes.length}/3000
              </p>
            </div>
            {validation ? (
              <p role="alert" className="text-sm text-danger">
                {validation}
              </p>
            ) : null}
            {presentation ? (
              <FormAlert
                title={presentation.title}
                description={presentation.message}
                code={isApiProblemError(error) ? error.code : undefined}
              />
            ) : null}
            {busy?.activeJobId ? (
              <Link
                to="/jobs/$jobId"
                params={{ jobId: busy.activeJobId }}
                className="text-sm text-accent underline"
              >
                查看进行中的生成任务
              </Link>
            ) : null}
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="secondary"
                disabled={generation.isPending}
                onClick={() => setOpen(false)}
              >
                取消
              </Button>
              <Button type="submit" disabled={generation.isPending}>
                {generation.isPending ? '创建任务中…' : '开始生成'}
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}
