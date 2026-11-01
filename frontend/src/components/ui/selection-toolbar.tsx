import { Button } from '@/components/ui/button'

// 与发布契约 BatchDelete.ids.maxItems 及后端批量删除上限一致。
const MAX_BATCH_DELETE_IDS = 100

export interface SelectionToolbarProps {
  /** 当前已加载条目总数（全选只作用于已加载项）。 */
  total: number
  /** 已选条目数。 */
  selected: number
  onToggleAll: (checked: boolean) => void
  onDelete: () => void
  onCancel: () => void
}

/**
 * 列表多选模式的工具栏：全选（支持半选）、已选计数、删除所选与取消。
 * 与整行 Link 列表搭配：进入选择模式后行改为 label + checkbox，不再跳转。
 */
export function SelectionToolbar({
  total,
  selected,
  onToggleAll,
  onDelete,
  onCancel,
}: SelectionToolbarProps) {
  const allSelected = total > 0 && selected === total
  const someSelected = selected > 0 && selected < total
  const exceedsLimit = selected > MAX_BATCH_DELETE_IDS

  return (
    <div className="flex flex-wrap items-center gap-3 rounded-md border border-border bg-surface-sunken px-4 py-2">
      <input
        type="checkbox"
        aria-label="全选"
        checked={allSelected}
        ref={(element) => {
          if (element) element.indeterminate = someSelected
        }}
        onChange={(event) => onToggleAll(event.target.checked)}
        className="shrink-0 accent-accent"
      />
      <span className="text-sm text-text-muted">已选 {selected} 项</span>
      <div className="ml-auto flex items-center gap-2">
        <Button
          variant="danger"
          size="sm"
          disabled={selected === 0 || exceedsLimit}
          onClick={onDelete}
        >
          删除所选
        </Button>
        <Button variant="secondary" size="sm" onClick={onCancel}>
          取消
        </Button>
      </div>
      {exceedsLimit ? (
        <p role="alert" className="w-full text-sm text-warn">
          一次最多删除 {MAX_BATCH_DELETE_IDS} 项，请减少选择。
        </p>
      ) : null}
    </div>
  )
}
