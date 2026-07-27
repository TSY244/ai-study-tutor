import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { BookOpen, ChevronRight } from 'lucide-react'
import { useState } from 'react'

import { listCourses } from '@/api/generated/endpoints/courses/courses'
import { listJobs } from '@/api/generated/endpoints/jobs/jobs'
import { getSystemStatus } from '@/api/generated/endpoints/system/system'
import type { SystemStatus } from '@/api/generated/model'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { PageHeader } from '@/components/ui/page-header'
import { Skeleton, SkeletonRows } from '@/components/ui/skeleton'
import { loadRecentLearning } from '@/features/learning/recent-learning'
import { formatDateTime } from '@/lib/format'
import { JOB_KIND_LABEL, JOB_STATUS_META, systemStatusMeta } from '@/lib/status'

/**
 * 学习总览：工作台首屏。继续学习、我的课程、进行中的任务、系统状态。
 * 数据均来自生成 Client + Query Cache（MSW curated fixtures 兜底）。
 */
export const Route = createFileRoute('/_shell/dashboard')({
  component: DashboardPage,
})

function DashboardPage() {
  return (
    <div className="space-y-5">
      <PageHeader title="学习总览" description="回到上次的学习，或查看正在后台运行的任务。" />
      <div className="grid gap-4 md:grid-cols-2">
        <ContinueLearningSection />
        <MyCoursesSection />
        <ActiveJobsSection />
        <SystemStatusSection />
      </div>
    </div>
  )
}

function SectionCard({
  id,
  title,
  children,
}: {
  id: string
  title: string
  children: React.ReactNode
}) {
  return (
    <section aria-labelledby={id} className="rounded-md border border-border bg-surface p-4">
      <h2 id={id} className="text-base font-medium text-text">
        {title}
      </h2>
      <div className="mt-3">{children}</div>
    </section>
  )
}

/** 最近访问记录属于纯客户端状态（localStorage），打开课时工作区时写入。 */
function ContinueLearningSection() {
  // 每次导航回本页组件都会重新挂载，挂载时读取即可拿到最新记录
  const [entries] = useState(() => loadRecentLearning())

  return (
    <SectionCard id="continue-learning-heading" title="继续学习">
      {entries.length === 0 ? (
        <EmptyState
          icon={BookOpen}
          title="还没有学习记录"
          description="打开一个课时开始学习后，这里会显示你的最近进度。"
          action={
            <Link
              to="/courses"
              className="rounded-sm text-sm font-medium text-accent hover:underline"
            >
              浏览课程
            </Link>
          }
        />
      ) : (
        <ul className="divide-y divide-border">
          {entries.map((entry) => (
            <li key={entry.lessonId}>
              <Link
                to="/courses/$courseId/lessons/$lessonId"
                params={{ courseId: entry.courseId, lessonId: entry.lessonId }}
                className="group flex items-center justify-between gap-3 py-2"
              >
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium text-text group-hover:text-accent">
                    {entry.lessonTitle}
                  </span>
                  <span className="text-xs text-text-subtle">
                    上次学习 {formatDateTime(entry.visitedAt)}
                  </span>
                </span>
                <ChevronRight
                  aria-hidden="true"
                  className="h-4 w-4 shrink-0 text-text-subtle group-hover:text-accent"
                />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  )
}

function MyCoursesSection() {
  const coursesQuery = useQuery({
    queryKey: queryKeys.courses({ limit: 3 }),
    queryFn: () => listCourses({ limit: 3 }),
  })

  return (
    <SectionCard id="my-courses-heading" title="我的课程">
      {coursesQuery.isPending ? (
        <div role="status" aria-busy="true" aria-label="课程加载中">
          <SkeletonRows rows={2} />
        </div>
      ) : null}
      {coursesQuery.isError ? (
        <ErrorState error={coursesQuery.error} onRetry={() => void coursesQuery.refetch()} />
      ) : null}
      {coursesQuery.isSuccess ? (
        coursesQuery.data.items.length === 0 ? (
          <EmptyState
            icon={BookOpen}
            title="还没有课程"
            description="创建第一门课程，然后导入课时开始学习。"
            action={
              <Link
                to="/courses/new"
                className="rounded-sm text-sm font-medium text-accent hover:underline"
              >
                新建课程
              </Link>
            }
          />
        ) : (
          <>
            <ul className="divide-y divide-border">
              {coursesQuery.data.items.map((course) => (
                <li key={course.id}>
                  <Link
                    to="/courses/$courseId"
                    params={{ courseId: course.id }}
                    className="group flex items-center justify-between gap-3 py-2"
                  >
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-medium text-text group-hover:text-accent">
                        {course.title}
                      </span>
                      <span className="text-xs text-text-subtle">
                        {course.lesson_count} 个课时 · 更新于 {formatDateTime(course.updated_at)}
                      </span>
                    </span>
                    <ChevronRight
                      aria-hidden="true"
                      className="h-4 w-4 shrink-0 text-text-subtle group-hover:text-accent"
                    />
                  </Link>
                </li>
              ))}
            </ul>
            <p className="mt-2 border-t border-border pt-2 text-right">
              <Link to="/courses" className="rounded-sm text-sm text-accent hover:underline">
                全部课程
              </Link>
            </p>
          </>
        )
      ) : null}
    </SectionCard>
  )
}

function ActiveJobsSection() {
  const jobsQuery = useQuery({
    queryKey: queryKeys.jobs({ active: true, limit: 3 }),
    queryFn: () => listJobs({ active: true, limit: 3 }),
  })

  return (
    <SectionCard id="active-jobs-heading" title="进行中的任务">
      {jobsQuery.isPending ? (
        <div role="status" aria-busy="true" aria-label="任务加载中">
          <SkeletonRows rows={2} />
        </div>
      ) : null}
      {jobsQuery.isError ? (
        <ErrorState error={jobsQuery.error} onRetry={() => void jobsQuery.refetch()} />
      ) : null}
      {jobsQuery.isSuccess ? (
        jobsQuery.data.items.length === 0 ? (
          <p className="text-sm text-text-muted">
            没有正在运行的任务。导入课时或生成测验后，进度会显示在这里和右上角任务中心。
          </p>
        ) : (
          <ul className="divide-y divide-border">
            {jobsQuery.data.items.map((job) => (
              <li key={job.id} className="py-2">
                <Link
                  to="/jobs/$jobId"
                  params={{ jobId: job.id }}
                  className="group flex items-center justify-between gap-3"
                >
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-medium text-text group-hover:text-accent">
                      {JOB_KIND_LABEL[job.kind]}
                    </span>
                    <span className="text-xs text-text-subtle">{job.stage}</span>
                  </span>
                  <Badge variant={JOB_STATUS_META[job.status].variant}>
                    {`${JOB_STATUS_META[job.status].label} ${job.progress}%`}
                  </Badge>
                </Link>
                <div
                  role="progressbar"
                  aria-valuenow={job.progress}
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-label={`${JOB_KIND_LABEL[job.kind]}进度`}
                  className="mt-1.5 h-1.5 w-full rounded-full bg-surface-sunken"
                >
                  <div
                    className="h-full rounded-full bg-info transition-[width] duration-180"
                    style={{ width: `${job.progress}%` }}
                  />
                </div>
              </li>
            ))}
          </ul>
        )
      ) : null}
    </SectionCard>
  )
}

function SystemStatusSection() {
  const statusQuery = useQuery({
    queryKey: queryKeys.systemStatus(),
    queryFn: () => getSystemStatus(),
  })

  return (
    <SectionCard id="system-status-heading" title="系统状态">
      {statusQuery.isPending ? (
        <div role="status" aria-busy="true" aria-label="系统状态加载中" className="space-y-2">
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-full" />
        </div>
      ) : null}
      {statusQuery.isError ? (
        <ErrorState
          title="无法获取系统状态"
          error={statusQuery.error}
          onRetry={() => void statusQuery.refetch()}
        />
      ) : null}
      {statusQuery.isSuccess ? <SystemStatusBody status={statusQuery.data} /> : null}
    </SectionCard>
  )
}

function SystemStatusBody({ status }: { status: SystemStatus }) {
  const overall = systemStatusMeta(status.status)
  const rows = [
    { name: '数据库', value: status.database.status },
    { name: '后台 Worker', value: status.worker.status },
    { name: 'Claude 连接', value: status.claude.status },
    { name: '本地存储', value: status.storage.status },
  ]
  return (
    <div>
      <div className="flex items-center justify-between gap-4">
        <Badge variant={overall.variant}>{overall.label}</Badge>
        <span className="text-sm text-text-subtle">版本 {status.version}</span>
      </div>
      <dl className="mt-3 divide-y divide-border text-sm">
        {rows.map((row) => (
          <div key={row.name} className="flex items-center justify-between gap-4 py-1.5">
            <dt className="text-text-muted">{row.name}</dt>
            <dd>
              <Badge variant={systemStatusMeta(row.value).variant}>
                {systemStatusMeta(row.value).label}
              </Badge>
            </dd>
          </div>
        ))}
      </dl>
      {status.maintenance.enabled ? (
        <p className="mt-3 text-sm text-warn">
          维护模式已开启{status.maintenance.reason ? `：${status.maintenance.reason}` : ''}
          ，写入操作可能暂不可用。
        </p>
      ) : null}
    </div>
  )
}
