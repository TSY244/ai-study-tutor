import { useInfiniteQuery, useQuery } from '@tanstack/react-query'

import {
  getCurrentDocument,
  listCurrentDocuments,
  listDocumentVersions,
} from '@/api/generated/endpoints/documents/documents'
import type { DocumentKind } from '@/api/generated/model'
import { queryKeys } from '@/api/query-client'

/** 当前文档集合（每 kind 一项）：驱动可用性与空状态文案。 */
export function useCurrentDocuments(lessonId: string) {
  return useQuery({
    queryKey: queryKeys.documents(lessonId),
    queryFn: () => listCurrentDocuments(lessonId),
  })
}

/** 指定 kind 的当前文档全文。404（未生成）由调用方按空状态处理。 */
export function useCurrentDocument(lessonId: string, kind: DocumentKind) {
  return useQuery({
    queryKey: queryKeys.document(lessonId, kind),
    queryFn: () => getCurrentDocument(lessonId, kind),
    // 正在阅读的文档不做后台轮询；更新通过任务完成后的精确失效触发。
    refetchOnWindowFocus: false,
  })
}

/** 历史版本列表：page_info 游标分页（cursor 是不透明值，只透传）。 */
export function useDocumentVersions(lessonId: string, kind: DocumentKind, enabled: boolean) {
  return useInfiniteQuery({
    queryKey: queryKeys.documentVersions(lessonId, kind),
    queryFn: ({ pageParam }) =>
      listDocumentVersions(lessonId, kind, { cursor: pageParam, limit: 10 }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) =>
      lastPage.page_info.has_more ? (lastPage.page_info.next_cursor ?? undefined) : undefined,
    enabled,
  })
}
