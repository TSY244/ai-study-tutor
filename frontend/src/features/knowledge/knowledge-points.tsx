import { DeleteContentButton } from '@/features/content/delete-content-button'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { listCourseKnowledgePoints } from '@/api/generated/endpoints/courses/courses'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

export function KnowledgePoints({ courseId, lessonId }: { courseId: string; lessonId?: string }) {
  const query = useQuery({
    queryKey: ['knowledge-points', courseId, lessonId ?? null],
    queryFn: () =>
      listCourseKnowledgePoints(courseId, lessonId ? { lesson_id: lessonId } : undefined),
  })
  const title = lessonId ? '本课时知识点' : '课程知识点'
  return (
    <section
      aria-label={title}
      className="space-y-3 rounded-lg border border-border bg-surface p-4"
    >
      <h2 className="text-base font-medium text-text">{title}</h2>
      {query.isPending ? <p className="text-sm text-text-muted">正在读取知识点…</p> : null}
      {query.isError ? (
        <div role="alert" className="text-sm text-danger">
          知识点加载失败。
          <Button size="sm" variant="secondary" onClick={() => void query.refetch()}>
            重试
          </Button>
        </div>
      ) : null}
      {query.data?.items.length === 0 ? (
        <p className="text-sm text-text-muted">
          首次生成测验时，AI 会先从资料中分析知识点并保存到课程。
        </p>
      ) : null}
      <ul className="space-y-2">
        {query.data?.items.map((point) => (
          <li key={point.id} className="flex items-start gap-3 rounded-md border border-border p-3">
            <details className="min-w-0 flex-1">
              <summary className="cursor-pointer text-sm font-medium text-text">
                {point.label}{' '}
                <Badge variant={point.importance === 'core' ? 'info' : 'neutral'}>
                  {point.importance === 'core' ? '核心' : '补充'}
                </Badge>
              </summary>
              <div className="mt-3 space-y-2 text-sm text-text-muted">
                <p>{point.explanation}</p>
                <p>
                  <span className="font-medium text-text">学习目标：</span>
                  {point.learning_objective}
                </p>
                <blockquote className="whitespace-pre-wrap border-l-2 border-border pl-3">
                  原文依据：{point.source_excerpt}
                </blockquote>
                {!lessonId ? (
                  <Link
                    to="/courses/$courseId/lessons/$lessonId/quizzes"
                    params={{ courseId, lessonId: point.lesson_id }}
                    className="text-accent underline"
                  >
                    {point.lesson_title} · 按知识点练习
                  </Link>
                ) : null}
              </div>
            </details>
            <DeleteContentButton
              kind="knowledge"
              id={point.id}
              name={point.label}
              courseId={courseId}
            />
          </li>
        ))}
      </ul>
    </section>
  )
}
