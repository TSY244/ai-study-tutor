import { useInfiniteQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { Target } from 'lucide-react'
import { useState } from 'react'

import { listWeaknesses } from '@/api/generated/endpoints/weaknesses/weaknesses'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { PageHeader } from '@/components/ui/page-header'
import { SkeletonRows } from '@/components/ui/skeleton'
import { cn } from '@/lib/cn'
import { formatDateTime } from '@/lib/format'

/**
 * 弱点列表：AI 从错题中归纳的薄弱知识点。支持按掌握状态过滤与
 * page_info 游标分页；复习/重开操作在后续阶段接入。
 */
export const Route = createFileRoute('/_shell/weaknesses')({
  component: WeaknessesPage,
})

type MasteredFilter = 'all' | 'active' | 'mastered'

const FILTERS: { value: MasteredFilter; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'active', label: '待复习' },
  { value: 'mastered', label: '已掌握' },
]

function WeaknessesPage() {
  const [filter, setFilter] = useState<MasteredFilter>('all')
  const mastered = filter === 'all' ? undefined : filter === 'mastered'

  const weaknessesQuery = useInfiniteQuery({
    queryKey: queryKeys.weaknesses({ mastered: filter }),
    queryFn: ({ pageParam }) =>
      listWeaknesses({
        cursor: pageParam,
        limit: 10,
        ...(mastered !== undefined ? { mastered } : {}),
      }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) =>
      lastPage.page_info.has_more ? (lastPage.page_info.next_cursor ?? undefined) : undefined,
  })

  const weaknesses = weaknessesQuery.data?.pages.flatMap((page) => page.items) ?? []

  return (
    <div className="space-y-5">
      <PageHeader
        title="弱点"
        description="系统从你的错题中归纳出的薄弱知识点，复习测验会优先覆盖它们。"
      />

      <div role="group" aria-label="按掌握状态筛选" className="flex gap-1">
        {FILTERS.map((item) => (
          <button
            key={item.value}
            type="button"
            aria-pressed={filter === item.value}
            onClick={() => setFilter(item.value)}
            className={cn(
              'rounded-md px-2.5 py-1 text-sm transition-colors duration-120',
              filter === item.value
                ? 'bg-accent-bg font-medium text-accent-strong'
                : 'text-text-muted hover:bg-surface-sunken hover:text-text',
            )}
          >
            {item.label}
          </button>
        ))}
      </div>

      {weaknessesQuery.isPending ? (
        <div role="status" aria-busy="true" aria-label="弱点加载中">
          <SkeletonRows rows={3} />
        </div>
      ) : null}

      {weaknessesQuery.isError ? (
        <ErrorState
          title="弱点加载失败"
          error={weaknessesQuery.error}
          onRetry={() => void weaknessesQuery.refetch()}
        />
      ) : null}

      {weaknessesQuery.isSuccess ? (
        weaknesses.length === 0 ? (
          <EmptyState
            icon={Target}
            title={filter === 'mastered' ? '还没有已掌握的弱点' : '还没有记录到弱点'}
            description={
              filter === 'all'
                ? '完成几次测验后，系统会在这里汇总你的薄弱知识点。'
                : '切换上方筛选条件查看其他状态的弱点。'
            }
            action={
              <Link
                to="/courses"
                className="rounded-sm text-sm font-medium text-accent hover:underline"
              >
                去课程学习
              </Link>
            }
          />
        ) : (
          <>
            <ul className="divide-y divide-border rounded-md border border-border bg-surface">
              {weaknesses.map((weakness) => (
                <li key={weakness.id} className="px-4 py-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium text-text">{weakness.concept_label}</span>
                    {weakness.mastered ? (
                      <Badge variant="ok">已掌握</Badge>
                    ) : (
                      <Badge variant="warn">待复习</Badge>
                    )}
                    <span className="text-xs text-text-subtle">
                      严重度 {weakness.severity}/5 · 已复习 {weakness.review_count} 次
                    </span>
                  </div>
                  <p className="mt-1 text-sm text-text-muted">{weakness.description}</p>
                  <p className="mt-1 text-xs text-text-subtle">
                    最近出现于 {formatDateTime(weakness.last_seen_at)}
                  </p>
                </li>
              ))}
            </ul>
            {weaknessesQuery.hasNextPage ? (
              <div>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={weaknessesQuery.isFetchingNextPage}
                  onClick={() => void weaknessesQuery.fetchNextPage()}
                >
                  {weaknessesQuery.isFetchingNextPage ? '加载中…' : '加载更多'}
                </Button>
              </div>
            ) : null}
          </>
        )
      ) : null}
    </div>
  )
}
