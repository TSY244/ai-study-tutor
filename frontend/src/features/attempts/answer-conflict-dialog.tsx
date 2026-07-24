import { useState } from 'react'

import type { Question } from '@/api/generated/model'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'

import type { AnswerSyncEngine, ConflictSnapshot } from './answer-sync-engine'
import { formatAnswerValue } from './answer-value'

/**
 * 答案冲突处理对话框（409 version_conflict 且无法安全合并时）。
 * 并排展示「我的未同步输入」与「服务端已保存版本」，由用户明确选择，
 * 绝不静默覆盖（handoff §6.4.6）。可暂时关闭（冲突保留，禁止提交），
 * 从状态条或题卡冲突入口重新打开。
 */
export function AnswerConflictDialog({
  engine,
  questions,
}: {
  engine: AnswerSyncEngine
  questions: Question[]
}) {
  const [dismissedKey, setDismissedKey] = useState<string | null>(null)
  const conflict = engine.getFirstConflict()
  const conflictKey = conflict
    ? `${conflict.questionId}:${conflict.serverAnswerVersion}:${conflict.serverAttemptVersion}`
    : null
  const open = conflict !== null && conflictKey !== dismissedKey

  if (!conflict) return null
  const question = questions.find((item) => item.id === conflict.questionId)

  const choose = (choice: 'local' | 'server') => {
    engine.resolveConflict(conflict.questionId, choice)
    setDismissedKey(null)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) setDismissedKey(conflictKey)
      }}
    >
      <DialogContent
        title="答案版本冲突"
        description={`「${question ? truncate(question.stem, 40) : '该题'}」在服务端已有一个已保存版本，与本机未同步的修改不同。请选择保留哪一个；未处理前不能提交。`}
      >
        <div className="space-y-3">
          <ConflictVersion
            heading="我的未同步输入"
            value={formatAnswerValue(question ?? fallbackQuestion(conflict), conflict.localValue)}
          />
          <ConflictVersion
            heading="服务端已保存版本"
            value={formatAnswerValue(question ?? fallbackQuestion(conflict), conflict.serverValue)}
          />
        </div>
        <div className="mt-4 flex flex-wrap justify-end gap-2">
          <Button variant="secondary" size="sm" onClick={() => choose('server')}>
            使用已保存版本
          </Button>
          <Button variant="primary" size="sm" onClick={() => choose('local')}>
            保留我的输入
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

function ConflictVersion({ heading, value }: { heading: string; value: string }) {
  return (
    <section className="rounded-md border border-border bg-surface-sunken p-3">
      <h3 className="text-xs font-medium text-text-subtle">{heading}</h3>
      <p className="mt-1 whitespace-pre-wrap break-words text-sm text-text">{value}</p>
    </section>
  )
}

/** formatAnswerValue 需要 Question；找不到时退化为仅展示原始值。 */
function fallbackQuestion(conflict: ConflictSnapshot): Question {
  return {
    id: conflict.questionId,
    type: 'short_answer',
    stem: '',
    options: null,
    tested_concepts: [],
    difficulty: 'medium',
    cognitive_level: 'explain',
    score: 0,
  }
}

function truncate(text: string, max: number): string {
  return text.length > max ? `${text.slice(0, max)}…` : text
}
