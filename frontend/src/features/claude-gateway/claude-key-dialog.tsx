import { Eye, EyeOff } from 'lucide-react'
import { useEffect, useState, type FormEvent } from 'react'

import type { ClaudeKey, ClaudeKeyCreate, ClaudeKeyUpdate } from '@/api/generated/model'
import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { FormAlert } from '@/components/ui/form-alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface ClaudeKeyDialogProps {
  open: boolean
  keyRecord: ClaudeKey | null
  onOpenChange: (open: boolean) => void
  onSave: (payload: ClaudeKeyCreate | ClaudeKeyUpdate) => Promise<void>
}

interface KeyDraft {
  protocol: ClaudeKey['protocol']
  name: string
  apiKey: string
  model: string
  baseUrl: string
  enabled: boolean
  priority: string
  weight: string
  timeoutSeconds: string
  cooldownSeconds: string
  maxConsecutiveFailures: string
}

const emptyDraft: KeyDraft = {
  protocol: 'anthropic',
  name: '',
  apiKey: '',
  model: '',
  baseUrl: '',
  enabled: true,
  priority: '10',
  weight: '100',
  timeoutSeconds: '300',
  cooldownSeconds: '300',
  maxConsecutiveFailures: '2',
}

function draftFor(keyRecord: ClaudeKey | null): KeyDraft {
  if (!keyRecord) return { ...emptyDraft }
  return {
    protocol: keyRecord.protocol ?? 'anthropic',
    name: keyRecord.name,
    apiKey: '',
    model: keyRecord.model,
    baseUrl: keyRecord.base_url ?? '',
    enabled: keyRecord.enabled,
    priority: String(keyRecord.priority),
    weight: String(keyRecord.weight),
    timeoutSeconds: String(keyRecord.timeout_seconds),
    cooldownSeconds: String(keyRecord.cooldown_seconds),
    maxConsecutiveFailures: String(keyRecord.max_consecutive_failures),
  }
}

function parseInteger(value: string, minimum: number, maximum: number): number | null {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed >= minimum && parsed <= maximum ? parsed : null
}

export function ClaudeKeyDialog({ open, keyRecord, onOpenChange, onSave }: ClaudeKeyDialogProps) {
  const editing = keyRecord !== null
  const [draft, setDraft] = useState<KeyDraft>(() => draftFor(keyRecord))
  const [showSecret, setShowSecret] = useState(false)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  useEffect(() => {
    if (open) {
      setDraft(draftFor(keyRecord))
      setShowSecret(false)
      setFormError(null)
    } else {
      // 完整 Key 只在对话框打开期间存在于受控输入状态中。
      setDraft((current) => ({ ...current, apiKey: '' }))
      setShowSecret(false)
    }
  }, [open, keyRecord])

  const change = <K extends keyof KeyDraft>(field: K, value: KeyDraft[K]) => {
    setDraft((current) => ({ ...current, [field]: value }))
  }

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setFormError(null)
    const name = draft.name.trim()
    const model = draft.model.trim()
    const baseUrl = draft.baseUrl.trim()
    const apiKey = draft.apiKey.trim()
    const priority = parseInteger(draft.priority, 0, Number.MAX_SAFE_INTEGER)
    const weight = parseInteger(draft.weight, 1, 1000)
    const timeoutSeconds = parseInteger(draft.timeoutSeconds, 1, 3600)
    const cooldownSeconds = parseInteger(draft.cooldownSeconds, 1, 86400)
    const maxConsecutiveFailures = parseInteger(draft.maxConsecutiveFailures, 1, 100)

    if (!name || name.length > 100) {
      setFormError('名称为必填项，且不能超过 100 个字符。')
      return
    }
    if (!model || model.length > 100) {
      setFormError('模型为必填项，且不能超过 100 个字符。')
      return
    }
    // 前端只做前缀校验；完整校验（https、无 userinfo、去尾 /）以后端为准。
    if (baseUrl !== '' && !baseUrl.startsWith('https://')) {
      setFormError('Base URL 必须是 https:// 开头的地址')
      return
    }
    if (!editing && !apiKey) {
      setFormError('请输入 API Key。')
      return
    }
    if (
      priority === null ||
      weight === null ||
      timeoutSeconds === null ||
      cooldownSeconds === null ||
      maxConsecutiveFailures === null
    ) {
      setFormError('路由参数超出允许范围，请检查优先级、权重、超时、冷却和失败阈值。')
      return
    }

    const common: ClaudeKeyUpdate = {
      protocol: draft.protocol,
      name,
      model,
      base_url: baseUrl,
      enabled: draft.enabled,
      priority,
      weight,
      timeout_seconds: timeoutSeconds,
      cooldown_seconds: cooldownSeconds,
      max_consecutive_failures: maxConsecutiveFailures,
    }
    const payload: ClaudeKeyCreate | ClaudeKeyUpdate = editing
      ? { ...common, ...(apiKey ? { api_key: apiKey } : {}) }
      : { ...common, api_key: apiKey, name, model }
    if (!editing && !payload.base_url) {
      // 创建时留空不发送 base_url；编辑时空串表示清除（回退环境变量）。
      delete payload.base_url
    }

    setSaving(true)
    try {
      await onSave(payload)
      setDraft((current) => ({ ...current, apiKey: '' }))
      onOpenChange(false)
    } catch (error) {
      const presentation = presentProblem(error)
      setFormError(`${presentation.title}：${presentation.message}`)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        title={editing ? '编辑模型 Key' : '添加模型 Key'}
        description={
          editing
            ? '密钥留空会保留原值；保存后 API 仍只返回脱敏视图。'
            : 'Key 将直接发送到本机 API 写入网关配置，不会保存到浏览器存储。'
        }
        className="max-w-2xl"
      >
        <form className="space-y-4" onSubmit={(event) => void submit(event)}>
          {formError ? <FormAlert title="无法保存模型 Key" description={formError} /> : null}

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="claude-key-name" required>
                名称
              </Label>
              <Input
                id="claude-key-name"
                required
                maxLength={100}
                autoComplete="off"
                value={draft.name}
                onChange={(event) => change('name', event.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="claude-key-model" required>
                模型
              </Label>
              <Input
                id="claude-key-model"
                required
                maxLength={100}
                autoComplete="off"
                placeholder={draft.protocol === 'openai' ? '模型 ID' : 'claude-sonnet-4-5'}
                value={draft.model}
                onChange={(event) => change('model', event.target.value)}
              />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="gateway-protocol">协议</Label>
            <select
              id="gateway-protocol"
              value={draft.protocol}
              onChange={(event) => change('protocol', event.target.value as KeyDraft['protocol'])}
              className="h-9 w-full rounded-md border border-border-strong bg-surface px-3 text-sm text-text"
            >
              <option value="anthropic">Anthropic（Messages）</option>
              <option value="openai">OpenAI（Chat Completions）</option>
            </select>
            <p className="text-xs text-text-subtle">
              修改后请重新应用到 Pi，让后续链接提取和测验使用新配置。
            </p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="claude-key-base-url">Base URL</Label>
            <Input
              id="claude-key-base-url"
              autoComplete="off"
              placeholder={
                draft.protocol === 'openai'
                  ? 'https://api.openai.com/v1'
                  : 'https://api.anthropic.com'
              }
              value={draft.baseUrl}
              onChange={(event) => change('baseUrl', event.target.value)}
            />
            <p className="text-xs text-text-subtle">
              {draft.protocol === 'openai'
                ? '留空使用 https://api.openai.com/v1；兼容网关请填写其 API 基础地址。'
                : '留空使用部署环境变量 ANTHROPIC_BASE_URL。'}
            </p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="claude-key-secret" required={!editing}>
              {editing ? 'API Key（可选）' : 'API Key'}
            </Label>
            <div className="flex gap-2">
              <Input
                id="claude-key-secret"
                type={showSecret ? 'text' : 'password'}
                required={!editing}
                autoComplete="new-password"
                placeholder={editing ? '留空则保留现有 Key' : '仅本次提交可见'}
                value={draft.apiKey}
                onChange={(event) => change('apiKey', event.target.value)}
              />
              <Button
                variant="secondary"
                size="sm"
                aria-label={showSecret ? '隐藏 API Key' : '显示 API Key'}
                aria-pressed={showSecret}
                onClick={() => setShowSecret((visible) => !visible)}
              >
                {showSecret ? (
                  <EyeOff aria-hidden="true" className="h-4 w-4" />
                ) : (
                  <Eye aria-hidden="true" className="h-4 w-4" />
                )}
              </Button>
            </div>
            <p className="text-xs text-text-subtle">
              已保存的完整 Key 不可回读；编辑时只有主动填写才会替换。
            </p>
          </div>

          <fieldset className="rounded-md border border-border p-3">
            <legend className="px-1 text-sm font-medium text-text">路由参数</legend>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              <NumberField
                id="claude-key-priority"
                label="优先级"
                min={0}
                value={draft.priority}
                onChange={(value) => change('priority', value)}
              />
              <NumberField
                id="claude-key-weight"
                label="权重"
                min={1}
                max={1000}
                value={draft.weight}
                onChange={(value) => change('weight', value)}
              />
              <NumberField
                id="claude-key-timeout"
                label="超时（秒）"
                min={1}
                max={3600}
                value={draft.timeoutSeconds}
                onChange={(value) => change('timeoutSeconds', value)}
              />
              <NumberField
                id="claude-key-cooldown"
                label="冷却（秒）"
                min={1}
                max={86400}
                value={draft.cooldownSeconds}
                onChange={(value) => change('cooldownSeconds', value)}
              />
              <NumberField
                id="claude-key-failures"
                label="连续失败阈值"
                min={1}
                max={100}
                value={draft.maxConsecutiveFailures}
                onChange={(value) => change('maxConsecutiveFailures', value)}
              />
            </div>
          </fieldset>

          <label className="flex items-center gap-2 text-sm text-text">
            <input
              type="checkbox"
              checked={draft.enabled}
              onChange={(event) => change('enabled', event.target.checked)}
            />
            启用此 Key
          </label>

          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              disabled={saving}
              onClick={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button type="submit" size="sm" disabled={saving}>
              {saving ? '保存中…' : editing ? '保存修改' : '创建 Key'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function NumberField({
  id,
  label,
  value,
  min,
  max,
  onChange,
}: {
  id: string
  label: string
  value: string
  min: number
  max?: number
  onChange: (value: string) => void
}) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Input
        id={id}
        type="number"
        required
        min={min}
        max={max}
        step={1}
        inputMode="numeric"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  )
}
