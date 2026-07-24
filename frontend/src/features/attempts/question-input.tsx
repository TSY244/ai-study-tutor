import type { Question } from '@/api/generated/model'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/cn'

import { toggleMultiValue } from './answer-value'

export interface QuestionInputProps {
  question: Question
  /** 当前值（引擎权威）；未作答为 undefined。 */
  value: unknown
  /** 提交中/已提交锁定，或该题有待处理冲突。 */
  disabled?: boolean
  onChange: (value: unknown) => void
  /** 失焦即 flush 该题（切题语义）。 */
  onBlur?: () => void
}

/**
 * 单题作答输入（handoff §9：语义化分组、选项完整可读标签）。
 * 选项 label 包裹 input + 选项全文；点击行内任意位置即可选择。
 */
export function QuestionInput({ question, value, disabled, onChange, onBlur }: QuestionInputProps) {
  const name = `answer-${question.id}`

  if (question.type === 'single_choice' || question.type === 'multiple_choice') {
    const options = Object.entries(question.options ?? {})
    const multiple = question.type === 'multiple_choice'
    return (
      <div
        role="group"
        aria-label={multiple ? '选项（多选）' : '选项（单选）'}
        className="space-y-1.5"
      >
        {options.map(([key, text]) => {
          const checked = multiple
            ? Array.isArray(value) && value.map(String).includes(key)
            : value === key
          return (
            <label
              key={key}
              className={cn(
                'flex cursor-pointer items-start gap-2.5 rounded-md border px-3 py-2 text-sm',
                checked ? 'border-accent bg-accent-bg text-text' : 'border-border text-text',
                disabled && 'cursor-not-allowed opacity-60',
              )}
            >
              <input
                type={multiple ? 'checkbox' : 'radio'}
                name={name}
                value={key}
                checked={checked}
                disabled={disabled}
                className="mt-0.5 shrink-0 accent-accent"
                onChange={(event) => {
                  if (multiple) {
                    onChange(toggleMultiValue(value, key, event.target.checked))
                  } else {
                    onChange(key)
                  }
                }}
                onBlur={onBlur}
              />
              <span className="min-w-0">
                <span className="font-medium">{key}.</span> {text}
              </span>
            </label>
          )
        })}
      </div>
    )
  }

  if (question.type === 'true_false') {
    const choices: Array<{ optionValue: boolean; label: string }> = [
      { optionValue: true, label: '正确' },
      { optionValue: false, label: '错误' },
    ]
    return (
      <div role="radiogroup" aria-label="判断对错" className="flex gap-2">
        {choices.map((choice) => {
          const checked = value === choice.optionValue
          return (
            <label
              key={String(choice.optionValue)}
              className={cn(
                'flex cursor-pointer items-center gap-2 rounded-md border px-4 py-2 text-sm',
                checked ? 'border-accent bg-accent-bg text-text' : 'border-border text-text',
                disabled && 'cursor-not-allowed opacity-60',
              )}
            >
              <input
                type="radio"
                name={name}
                checked={checked}
                disabled={disabled}
                className="accent-accent"
                onChange={() => onChange(choice.optionValue)}
                onBlur={onBlur}
              />
              {choice.label}
            </label>
          )
        })}
      </div>
    )
  }

  // short_answer / case_analysis：自由文本
  return (
    <Textarea
      id={name}
      aria-label="作答内容"
      rows={question.type === 'case_analysis' ? 7 : 3}
      value={typeof value === 'string' ? value : ''}
      disabled={disabled}
      placeholder="输入你的作答…"
      onChange={(event) => onChange(event.target.value)}
      onBlur={onBlur}
    />
  )
}
