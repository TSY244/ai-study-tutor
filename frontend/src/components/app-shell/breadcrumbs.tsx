import { useQuery } from '@tanstack/react-query'
import { useMatches } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'

import { getCourse } from '@/api/generated/endpoints/courses/courses'
import { getLesson } from '@/api/generated/endpoints/lessons/lessons'
import { queryKeys } from '@/api/query-client'
import { PathLink } from '@/components/ui/path-link'
import { Skeleton } from '@/components/ui/skeleton'

declare module '@tanstack/react-router' {
  interface StaticDataRouteOption {
    /**
     * 面包屑标签。'$course' / '$lesson' 为占位符，由 Breadcrumbs
     * 用共享 Query Cache 中的实体标题替换（与页面同源，无额外请求）。
     */
    crumb?: string
  }
}

/**
 * 面包屑：由当前路由匹配链的 staticData.crumb 生成。动态段（课程/课时）
 * 先显示骨架占位，标题从 Query Cache 就绪后原地替换，不移动其他 crumb。
 * 仅一级 crumb 时不渲染（顶层页面不需要）。
 */
export function Breadcrumbs() {
  const matches = useMatches()
  const params = (matches[matches.length - 1]?.params ?? {}) as {
    courseId?: string
    lessonId?: string
  }

  const courseQuery = useQuery({
    queryKey: queryKeys.course(params.courseId ?? ''),
    queryFn: () => getCourse(params.courseId as string),
    enabled: Boolean(params.courseId),
  })
  const lessonQuery = useQuery({
    queryKey: queryKeys.lesson(params.lessonId ?? ''),
    queryFn: () => getLesson(params.lessonId as string),
    enabled: Boolean(params.lessonId),
  })

  const items = matches
    .filter((match) => typeof match.staticData?.crumb === 'string')
    .map((match) => ({ label: match.staticData.crumb as string, pathname: match.pathname }))

  if (items.length <= 1) return null

  return (
    <nav aria-label="面包屑" className="border-b border-border">
      <ol className="mx-auto flex max-w-6xl items-center gap-1 overflow-x-auto px-4 py-2 text-sm xl:max-w-7xl 2xl:max-w-[92rem]">
        {items.map((item, index) => {
          const isLast = index === items.length - 1
          const label = (
            <CrumbLabel
              item={item}
              courseTitle={courseQuery.data?.title}
              lessonTitle={lessonQuery.data?.title}
            />
          )
          return (
            <li key={item.pathname} className="flex shrink-0 items-center gap-1">
              {index > 0 ? (
                <ChevronRight aria-hidden="true" className="h-3.5 w-3.5 text-text-subtle" />
              ) : null}
              {isLast ? (
                <span aria-current="page" className="font-medium text-text">
                  {label}
                </span>
              ) : (
                <PathLink to={item.pathname} className="rounded-sm text-text-muted hover:text-text">
                  {label}
                </PathLink>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}

function CrumbLabel({
  item,
  courseTitle,
  lessonTitle,
}: {
  item: { label: string; pathname: string }
  courseTitle?: string
  lessonTitle?: string
}) {
  if (item.label === '$course') {
    return courseTitle ?? <Skeleton className="inline-block h-3.5 w-20 align-middle" />
  }
  if (item.label === '$lesson') {
    return lessonTitle ?? <Skeleton className="inline-block h-3.5 w-24 align-middle" />
  }
  return <>{item.label}</>
}
