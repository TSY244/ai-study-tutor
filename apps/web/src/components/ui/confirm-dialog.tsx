import { Dialog, DialogContent } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

export interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  /** 确认按钮文案，如“归档”“删除”。 */
  confirmLabel: string
  danger?: boolean
  /** 异步确认进行中：禁用两个按钮，防重复提交。 */
  busy?: boolean
  /** 上一次确认失败的错误说明（保留对话框，不清空上下文）。 */
  errorMessage?: string
  onConfirm: () => void
}

/**
 * 破坏/状态变更操作的统一确认框（Radix Dialog：焦点圈禁、Esc 关闭、
 * 关闭后焦点回触发器）。初始焦点落在“取消”上——破坏性操作要求用户
 * 主动移动到确认按钮。
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  danger = false,
  busy = false,
  errorMessage,
  onConfirm,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent title={title} description={description}>
        {errorMessage ? (
          <p role="alert" className="mb-3 rounded-md bg-danger-bg px-3 py-2 text-sm text-danger">
            {errorMessage}
          </p>
        ) : null}
        <div className="flex justify-end gap-2">
          <Button variant="secondary" size="sm" disabled={busy} onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            variant={danger ? 'danger' : 'primary'}
            size="sm"
            disabled={busy}
            onClick={onConfirm}
          >
            {busy ? `${confirmLabel}中…` : confirmLabel}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
