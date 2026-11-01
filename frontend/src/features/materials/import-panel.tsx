import { Content, List, Root, Trigger } from '@radix-ui/react-tabs'

import type { Lesson } from '@/api/generated/model'
import { ActiveJobCard } from '@/features/materials/active-job-card'
import { TextImportForm } from '@/features/materials/text-import-form'
import { UploadImportForm } from '@/features/materials/upload-import-form'
import { UrlImportForm } from '@/features/materials/url-import-form'
import { cn } from '@/lib/cn'

const SOURCES = [
  { value: 'text', label: '文本' },
  { value: 'url', label: '链接' },
  { value: 'upload', label: '文件' },
  { value: 'audio', label: '音频' },
] as const

/**
 * 素材导入面板（02 §6.1：三种来源同一创建语义）：
 * - draft/failed 课时显示导入表单（Radix Tabs，键盘方向键可达）；
 * - processing 课时显示进行中的生成任务卡；
 * - ready 课时不显示（内容视图在子路由）。
 */
export function ImportPanel({ lesson }: { lesson: Lesson }) {
  if (lesson.status === 'processing' && lesson.active_job_id) {
    return (
      <ActiveJobCard
        jobId={lesson.active_job_id}
        lessonId={lesson.id}
        courseId={lesson.course_id}
      />
    )
  }

  if (lesson.status !== 'draft' && lesson.status !== 'failed') return null

  return (
    <section
      aria-labelledby="import-panel-heading"
      className="rounded-md border border-border bg-surface p-4"
    >
      <h2 id="import-panel-heading" className="text-base font-medium text-text">
        导入素材
      </h2>
      <p className="mt-1 text-sm text-text-muted">
        文本、链接、图片、扫描 PDF、PPTX 或音频均可导入。先提取原文，再由 AI
        总结并生成笔记与教纲；离开页面任务不中断。
      </p>
      {lesson.status === 'failed' ? (
        <p className="mt-2 rounded-md bg-warn-bg px-3 py-2 text-sm text-warn">
          上次生成失败。可以换一种来源重新导入，或先到任务中心查看失败原因。
        </p>
      ) : null}

      <Root defaultValue="text" className="mt-4">
        <List aria-label="素材来源" className="flex items-center gap-1 border-b border-border">
          {SOURCES.map((source) => (
            <Trigger
              key={source.value}
              value={source.value}
              className={cn(
                '-mb-px border-b-2 border-transparent px-3 py-2 text-sm text-text-muted',
                'hover:text-text data-[state=active]:border-accent data-[state=active]:font-medium data-[state=active]:text-accent-strong',
              )}
            >
              {source.label}
            </Trigger>
          ))}
        </List>
        <Content value="text" className="pt-4">
          <TextImportForm lesson={lesson} />
        </Content>
        <Content value="url" className="pt-4">
          <UrlImportForm lesson={lesson} />
        </Content>
        <Content value="upload" className="pt-4">
          <UploadImportForm lesson={lesson} />
        </Content>
        <Content value="audio" className="pt-4">
          <UploadImportForm lesson={lesson} audioOnly />
        </Content>
      </Root>
    </section>
  )
}
