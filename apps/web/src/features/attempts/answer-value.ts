import type { Question, QuestionType } from '@/api/generated/model'

/**
 * 答案值（AnswerValue.value）的前端约定与比较工具。
 *
 * 取值约定（前端与 MSW 判分共用；后端按契约把 value 视为不透明 JSON）：
 * - single_choice：选项键，如 "B"
 * - multiple_choice：选项键数组，存储/比较前排序
 * - true_false：boolean
 * - short_answer / case_analysis：字符串
 */

/** 客观题：可本地判分（单选/多选/判断），提交走 200 分支的判定依据。 */
export function isObjectiveQuestion(type: QuestionType): boolean {
  return type === 'single_choice' || type === 'multiple_choice' || type === 'true_false'
}

/** 规范化：多选数组排序，使 ["A","B"] 与 ["B","A"] 等价。 */
export function normalizeAnswerValue(value: unknown): unknown {
  if (Array.isArray(value)) return [...value].map(String).sort()
  return value
}

/** 深度相等（JSON 语义）；多选按无序集合比较。 */
export function answerValuesEqual(a: unknown, b: unknown): boolean {
  return JSON.stringify(normalizeAnswerValue(a)) === JSON.stringify(normalizeAnswerValue(b))
}

/** 该题是否已有有效作答（用于「已答 x/y」进度与提交前确认）。 */
export function hasAnswerValue(question: Question, value: unknown): boolean {
  if (value === undefined || value === null) return false
  if (question.type === 'multiple_choice') return Array.isArray(value) && value.length > 0
  if (typeof value === 'string') return value.trim().length > 0
  return true
}

/** 多选切换：返回排序后的新数组。 */
export function toggleMultiValue(current: unknown, optionKey: string, checked: boolean): string[] {
  const list = Array.isArray(current) ? current.map(String) : []
  const next = checked ? [...list, optionKey] : list.filter((key) => key !== optionKey)
  return next.sort()
}

/** 把答案值渲染为可读文本（冲突对比、结果页展示）。 */
export function formatAnswerValue(question: Question, value: unknown): string {
  if (value === undefined || value === null) return '（未作答）'
  if (question.type === 'true_false') {
    if (value === true || value === 'true') return '正确'
    if (value === false || value === 'false') return '错误'
  }
  if (Array.isArray(value)) {
    if (value.length === 0) return '（未作答）'
    const labels = value.map((key) => optionLabel(question, String(key)))
    return labels.join('、')
  }
  if (typeof value === 'string') {
    if (value.trim().length === 0) return '（未作答）'
    return question.type === 'single_choice' ? optionLabel(question, value) : value
  }
  return String(value)
}

function optionLabel(question: Question, key: string): string {
  const text = question.options?.[key]
  return text ? `${key}. ${text}` : key
}

/** 答案值在 UI 中的初始读取：服务端 AttemptAnswer / 草稿统一走这里。 */
export function initialValueFor(question: Question, value: unknown): unknown {
  if (value !== undefined) return value
  if (question.type === 'multiple_choice') return []
  if (question.type === 'short_answer' || question.type === 'case_analysis') return ''
  return undefined
}
