import { DeleteContentButton } from '@/features/content/delete-content-button'
import { MaterialList } from '@/features/materials/material-list'
import { FileText, History } from 'lucide-react'
import { useState } from 'react'

import type { Document, DocumentKind } from '@/api/generated/model'
import { isApiProblemError } from '@/api/problem'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { SkeletonText } from '@/components/ui/skeleton'
import { DocumentVersionsDialog } from '@/features/documents/document-versions-dialog'
import { MarkdownReader } from '@/features/documents/markdown/markdown-reader'
import { useCurrentDocument, useCurrentDocuments } from '@/features/documents/use-documents'
import { ResourceJobs } from '@/features/jobs/resource-jobs'
import { useActiveResourceJobs } from '@/features/jobs/use-active-resource-jobs'
import { formatDateTime } from '@/lib/format'
import { DOCUMENT_KIND_LABEL } from '@/lib/status'

/**
 * 文档阅读视图（笔记/原文/大纲共用）。
 *
 * 阅读连续性（02-frontend-handoff §6.3）：首次加载后固定当前阅读版本
 * （pinned）；后台检测到新版本（生成任务完成后的精确失效触发 refetch）
 * 只显示"内容已更新"提示，不替换内容、不改变滚动位置，由用户决定何时切换。
 * 历史版本以只读快照查看，可随时回到当前版本。
 */
export function DocumentView({ lessonId, kind }: { lessonId: string; kind: DocumentKind }) {
  const jobsQuery = useActiveResourceJobs('lesson', lessonId, [
    'lesson_import',
    'lesson_regenerate',
  ])

  return (
    <div className="space-y-4">
      <ResourceJobs
        query={{
          ...jobsQuery,
          // 导入任务由课时布局中的 ImportPanel 在标签栏下统一展示。
          jobs: jobsQuery.jobs.filter((job) => job.kind !== 'lesson_import'),
        }}
        title={`${DOCUMENT_KIND_LABEL[kind]}正在生成`}
      />
      <DocumentContent
        lessonId={lessonId}
        kind={kind}
        generating={jobsQuery.isLoading || jobsQuery.jobs.length > 0}
      />
      {kind === 'raw_text' ? <MaterialList lessonId={lessonId} /> : null}
    </div>
  )
}

function DocumentContent({
  lessonId,
  kind,
  generating,
}: {
  lessonId: string
  kind: DocumentKind
  generating: boolean
}) {
  const docQuery = useCurrentDocument(lessonId, kind)
  const [pinned, setPinned] = useState<Document | null>(null)
  const [historical, setHistorical] = useState<Document | null>(null)
  const [dismissedUpdateId, setDismissedUpdateId] = useState<string | null>(null)
  const [versionsOpen, setVersionsOpen] = useState(false)

  // 首次拿到当前文档时固定为阅读版本（渲染期调整 state，仅执行一次）。
  if (docQuery.isSuccess && docQuery.data && pinned === null) {
    setPinned(docQuery.data)
  }

  if (docQuery.isPending) {
    return (
      <div role="status" aria-busy="true" aria-label="文档加载中" className="space-y-3">
        <SkeletonText lines={8} />
      </div>
    )
  }

  if (docQuery.isError) {
    if (isApiProblemError(docQuery.error) && docQuery.error.status === 404) {
      if (!historical)
        return (
          <div className="space-y-3">
            {generating ? null : <DocumentEmptyState lessonId={lessonId} kind={kind} />}
            <Button variant="ghost" size="sm" onClick={() => setVersionsOpen(true)}>
              <History aria-hidden="true" className="h-4 w-4" />
              历史版本
            </Button>
            <DocumentVersionsDialog
              lessonId={lessonId}
              kind={kind}
              open={versionsOpen}
              onOpenChange={setVersionsOpen}
              onView={(doc) => {
                setVersionsOpen(false)
                setHistorical(doc)
              }}
            />
          </div>
        )
    } else
      return (
        <ErrorState
          title={`${DOCUMENT_KIND_LABEL[kind]}加载失败`}
          error={docQuery.error}
          onRetry={() => void docQuery.refetch()}
        />
      )
  }

  const latest = docQuery.isSuccess ? docQuery.data : null
  const displayed = historical ?? pinned
  if (!displayed) return null

  const updateAvailable =
    latest !== null &&
    historical === null &&
    latest.id !== displayed.id &&
    dismissedUpdateId !== latest.id

  function viewVersion(doc: Document) {
    setVersionsOpen(false)
    setHistorical(doc.id === pinned?.id ? null : doc)
  }

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="flex flex-wrap items-center gap-2 text-sm text-text-muted">
          <span>{DOCUMENT_KIND_LABEL[kind]}</span>
          <Badge variant="neutral">{`R${displayed.revision}`}</Badge>
          {historical ? <Badge variant="warn">历史版本</Badge> : null}
          <span className="text-xs text-text-subtle">
            更新于 {formatDateTime(displayed.created_at)}
          </span>
        </p>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={() => setVersionsOpen(true)}>
            <History aria-hidden="true" className="h-4 w-4" />
            历史版本
          </Button>
          <DeleteContentButton
            kind="document"
            id={displayed.id}
            name={`${DOCUMENT_KIND_LABEL[kind]} R${displayed.revision}`}
            lessonId={lessonId}
            onDeleted={() => {
              setPinned(null)
              setHistorical(null)
              setDismissedUpdateId(null)
            }}
          />
        </div>
      </div>

      {updateAvailable && latest ? (
        <div
          role="status"
          className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md border border-info/30 bg-info-bg px-3 py-2 text-sm text-text"
        >
          <span>内容已更新：新版本 R{latest.revision} 已发布，你当前的阅读位置保持不变。</span>
          <span className="flex gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                setPinned(latest)
                setDismissedUpdateId(null)
              }}
            >
              查看新版本
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setDismissedUpdateId(latest.id)}>
              继续阅读当前版本
            </Button>
          </span>
        </div>
      ) : null}

      {historical ? (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md border border-warn/30 bg-warn-bg px-3 py-2 text-sm text-text">
          <span>
            正在查看历史版本 R{historical.revision}（{formatDateTime(historical.created_at)}
            ），内容为只读快照。
          </span>
          <Button variant="secondary" size="sm" onClick={() => setHistorical(null)}>
            回到当前版本
          </Button>
        </div>
      ) : null}

      <MarkdownReader markdown={displayed.content_md} />

      <DocumentVersionsDialog
        lessonId={lessonId}
        kind={kind}
        open={versionsOpen}
        onOpenChange={setVersionsOpen}
        onView={viewVersion}
      />
    </div>
  )
}

/**
 * 文档未生成（404）时的空状态。用 DocumentSummaryCollection 说明哪些
 * 类型已就绪，引导到已有内容；进行中的任务由上方任务卡显示。
 */
function DocumentEmptyState({ lessonId, kind }: { lessonId: string; kind: DocumentKind }) {
  const collectionQuery = useCurrentDocuments(lessonId)
  const label = DOCUMENT_KIND_LABEL[kind]
  const availableKinds = (collectionQuery.data?.items ?? [])
    .filter((item) => item.kind !== kind)
    .map((item) => DOCUMENT_KIND_LABEL[item.kind])

  return (
    <EmptyState
      icon={FileText}
      title={`${label}尚未生成`}
      description={
        availableKinds.length > 0
          ? `${availableKinds.join('、')}已就绪，可通过上方标签切换查看；生成任务完成后这里会显示${label}。`
          : `导入素材并等待生成任务完成后，这里会显示${label}。任务进度可在右下角任务中心查看。`
      }
    />
  )
}
