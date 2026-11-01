import { useQuery } from '@tanstack/react-query'

import { listJobs } from '@/api/generated/endpoints/jobs/jobs'
import type { JobKind } from '@/api/generated/model'
import { queryKeys } from '@/api/query-client'
import { isJobActive } from '@/lib/status'

/** 按资源查询活动任务，沿用全局 SSE 的进度合并与终态失效。 */
export function useActiveResourceJobs(
  aggregateType: string,
  aggregateId: string | null | undefined,
  kinds: readonly JobKind[],
) {
  const filters = {
    active: true,
    aggregate_type: aggregateType,
    aggregate_id: aggregateId ?? undefined,
    limit: 100,
  }
  const query = useQuery({
    queryKey: queryKeys.jobs(filters),
    queryFn: async () => {
      const result = await listJobs(filters)
      let page = result
      while (page.page_info.has_more && page.page_info.next_cursor) {
        page = await listJobs({ ...filters, cursor: page.page_info.next_cursor })
        result.items.push(...page.items)
      }
      return { ...result, page_info: page.page_info }
    },
    enabled: aggregateId !== null,
  })

  return {
    ...query,
    // SSE 会先把终态合并入缓存，再重取活动列表；此间也不显示已结束的任务。
    jobs: (query.data?.items ?? []).filter(
      (job) =>
        job.aggregate_type === aggregateType &&
        (aggregateId === undefined || job.aggregate_id === aggregateId) &&
        kinds.includes(job.kind) &&
        isJobActive(job.status),
    ),
  }
}
