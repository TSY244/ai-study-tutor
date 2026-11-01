import { useInfiniteQuery } from '@tanstack/react-query'
import { listMaterials } from '@/api/generated/endpoints/materials/materials'
import { Button } from '@/components/ui/button'
import { ErrorState } from '@/components/ui/error-state'
import { DeleteContentButton } from '@/features/content/delete-content-button'
import { formatDateTime } from '@/lib/format'

export function MaterialList({ lessonId }: { lessonId: string }) {
  const query = useInfiniteQuery({
    queryKey: ['materials', lessonId],
    queryFn: ({ pageParam }) => listMaterials(lessonId, { cursor: pageParam, limit: 20 }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) =>
      page.page_info.has_more ? (page.page_info.next_cursor ?? undefined) : undefined,
  })
  return (
    <section
      aria-label="来源素材"
      className="space-y-3 rounded-lg border border-border bg-surface p-4"
    >
      <h2 className="text-base font-medium text-text">来源素材</h2>
      {query.isPending ? <p className="text-sm text-text-muted">正在读取素材…</p> : null}
      {query.isError ? (
        <ErrorState title="素材加载失败" error={query.error} onRetry={() => void query.refetch()} />
      ) : null}
      {query.data?.pages[0]?.items.length === 0 ? (
        <p className="text-sm text-text-muted">暂无来源素材。</p>
      ) : null}
      <ul className="divide-y divide-border">
        {query.data?.pages
          .flatMap((page) => page.items)
          .map((material) => {
            const name = material.original_name || material.source_url || '文本素材'
            return (
              <li key={material.id} className="flex items-center gap-3 py-2">
                <div className="min-w-0 flex-1">
                  <p className="break-all text-sm text-text">{name}</p>
                  <p className="text-xs text-text-subtle">{formatDateTime(material.created_at)}</p>
                </div>
                <DeleteContentButton
                  kind="material"
                  id={material.id}
                  name={name}
                  lessonId={lessonId}
                />
              </li>
            )
          })}
      </ul>
      {query.hasNextPage ? (
        <Button
          variant="secondary"
          size="sm"
          disabled={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
        >
          {query.isFetchingNextPage ? '加载中…' : '加载更多素材'}
        </Button>
      ) : null}
    </section>
  )
}
