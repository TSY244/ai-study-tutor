import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { listLessons } from '@/api/generated/endpoints/lessons/lessons'
import { queryKeys } from '@/api/query-client'
import { ErrorState } from '@/components/ui/error-state'
import { PageHeader } from '@/components/ui/page-header'
import { Skeleton, SkeletonText } from '@/components/ui/skeleton'
import { NewLessonForm } from '@/features/lessons/new-lesson-form'

export const Route = createFileRoute('/_shell/courses/$courseId/lessons/new')({
  staticData: { crumb: '新建课时' },
  component: NewLessonPage,
})

/** 新建课时：课次默认取当前最大课次 +1，可改。 */
function NewLessonPage() {
  const { courseId } = Route.useParams()
  const lessonsQuery = useQuery({
    queryKey: [...queryKeys.lessons(courseId), { view: 'next-ordinal' }] as const,
    queryFn: () => listLessons(courseId, { limit: 100 }),
  })

  const nextOrdinal =
    (lessonsQuery.data?.items.reduce((max, lesson) => Math.max(max, lesson.ordinal), 0) ?? 0) + 1

  return (
    <div className="space-y-5">
      <PageHeader title="新建课时" description="创建空课时后，通过文本、链接或文件导入素材。" />
      {lessonsQuery.isPending ? (
        <div role="status" aria-busy="true" aria-label="表单加载中" className="max-w-xl space-y-4">
          <Skeleton className="h-9 w-full" />
          <SkeletonText lines={2} />
        </div>
      ) : null}
      {lessonsQuery.isError ? (
        <ErrorState
          title="课时信息加载失败"
          error={lessonsQuery.error}
          onRetry={() => void lessonsQuery.refetch()}
        />
      ) : null}
      {lessonsQuery.isSuccess ? (
        // 课次默认值就绪后再挂载表单（defaultValues 只取一次）
        <NewLessonForm courseId={courseId} nextOrdinal={nextOrdinal} />
      ) : null}
    </div>
  )
}
