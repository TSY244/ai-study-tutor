import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { getDocumentVersion } from '@/api/generated/endpoints/documents/documents'
import type { Document, DocumentKind } from '@/api/generated/model'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { SkeletonRows } from '@/components/ui/skeleton'
import { useDocumentVersions } from '@/features/documents/use-documents'
import { formatDateTime } from '@/lib/format'

/**
 * 文档历史版本对话框：page_info 游标分页（"加载更多"），选择某个版本后
 * 拉取全文交给父组件以只读快照展示。版本内容不可变，按 documentId 缓存。
 */
export function DocumentVersionsDialog({
  lessonId,
  kind,
  open,
  onOpenChange,
  onView,
}: {
  lessonId: string
  kind: DocumentKind
  open: boolean
  onOpenChange: (open: boolean) => void
  onView: (doc: Document) => void
}) {
  const queryClient = useQueryClient()
  const versionsQuery = useDocumentVersions(lessonId, kind, open)
  const [loadingId, setLoadingId] = useState<string | null>(null)

  async function viewVersion(documentId: string) {
    setLoadingId(documentId)
    try {
      const doc = await queryClient.fetchQuery({
        queryKey: queryKeys.documentVersion(documentId),
        queryFn: () => getDocumentVersion(documentId),
        staleTime: Number.POSITIVE_INFINITY, // 版本快照不可变
      })
      onView(doc)
    } finally {
      setLoadingId(null)
    }
  }

  const versions = versionsQuery.data?.pages.flatMap((page) => page.items) ?? []

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        title="历史版本"
        description="版本按从新到旧排列；查看历史版本不会离开当前页面。"
      >
        {versionsQuery.isPending ? (
          <div role="status" aria-busy="true" aria-label="版本列表加载中">
            <SkeletonRows rows={3} />
          </div>
        ) : null}

        {versionsQuery.isError ? (
          <p role="alert" className="text-sm text-text-muted">
            版本列表加载失败。
            <button
              type="button"
              onClick={() => void versionsQuery.refetch()}
              className="ml-1 rounded-sm font-medium text-accent hover:underline"
            >
              重试
            </button>
          </p>
        ) : null}

        {versionsQuery.isSuccess ? (
          <>
            <ul className="divide-y divide-border">
              {versions.map((version) => (
                <li key={version.id} className="flex items-center justify-between gap-3 py-2.5">
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <span className="text-sm font-medium text-text">R{version.revision}</span>
                    {version.is_active ? <Badge variant="accent">当前版本</Badge> : null}
                    <span className="text-xs text-text-subtle">
                      {formatDateTime(version.created_at)}
                    </span>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={loadingId !== null}
                    onClick={() => void viewVersion(version.id)}
                  >
                    {loadingId === version.id ? '加载中…' : '查看'}
                  </Button>
                </li>
              ))}
            </ul>
            {versionsQuery.hasNextPage ? (
              <div className="pt-3">
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={versionsQuery.isFetchingNextPage}
                  onClick={() => void versionsQuery.fetchNextPage()}
                >
                  {versionsQuery.isFetchingNextPage ? '加载中…' : '加载更多'}
                </Button>
              </div>
            ) : null}
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
