import { useEffect, useState, type FormEvent } from 'react'

import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { FormAlert } from '@/components/ui/form-alert'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

interface BilibiliCookieDialogProps {
  open: boolean
  /** true 表示替换已保存的 Cookie（提交覆盖）。 */
  replacing: boolean
  onOpenChange: (open: boolean) => void
  onSave: (content: string) => Promise<void>
}

const MAX_CONTENT_LENGTH = 65_536

/**
 * 保存 B 站 Cookie 对话框。完整内容只在打开期间存在于受控状态，
 * 提交直送本机 API；关闭即清空。
 */
export function BilibiliCookieDialog({
  open,
  replacing,
  onOpenChange,
  onSave,
}: BilibiliCookieDialogProps) {
  const [content, setContent] = useState('')
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  useEffect(() => {
    if (open) {
      setContent('')
      setFormError(null)
    } else {
      // 完整 Cookie 只在对话框打开期间存在于受控输入状态中。
      setContent('')
    }
  }, [open])

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setFormError(null)
    const trimmed = content.trim()
    if (!trimmed) {
      setFormError('请粘贴 Cookie 内容或 SESSDATA 值。')
      return
    }
    if (trimmed.length > MAX_CONTENT_LENGTH) {
      setFormError('内容超过 64 KiB 上限。请只粘贴 bilibili.com 相关的 Cookie 或 SESSDATA。')
      return
    }
    setSaving(true)
    try {
      await onSave(trimmed)
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
        title={replacing ? '替换 B 站 Cookie' : '保存 B 站 Cookie'}
        description="内容直送本机 API 写入部署（0600 权限），不会保存到浏览器存储；仅保留 bilibili.com 域名条目。"
        className="max-w-2xl"
      >
        <form className="space-y-4" onSubmit={(event) => void submit(event)}>
          {formError ? <FormAlert title="无法保存 B 站 Cookie" description={formError} /> : null}

          <div className="space-y-1.5">
            <Label htmlFor="bilibili-cookie-content" required>
              Cookie 内容
            </Label>
            <Textarea
              id="bilibili-cookie-content"
              required
              rows={6}
              maxLength={MAX_CONTENT_LENGTH}
              autoComplete="off"
              spellCheck={false}
              placeholder={'SESSDATA=… 或直接粘贴 SESSDATA 值，或 Netscape Cookie 导出内容'}
              value={content}
              onChange={(event) => setContent(event.target.value)}
            />
            <p className="text-xs text-text-subtle">
              获取方式：登录 bilibili.com 后，从浏览器开发者工具（应用 → Cookie）复制
              SESSDATA；或粘贴「SESSDATA=xxx; …」Cookie 头串 / 导出的 Netscape Cookie 文件内容。
              保存后建议立即执行一次「检测可用性」。
            </p>
          </div>

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
              {saving ? '保存中…' : replacing ? '覆盖保存' : '保存'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
