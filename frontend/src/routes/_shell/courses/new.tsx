import { Content, List, Root, Trigger } from '@radix-ui/react-tabs'
import { FileCourseForm } from '@/features/courses/file-course-form'
import { AudioCourseForm } from '@/features/courses/audio-course-form'
import { createFileRoute } from '@tanstack/react-router'

import { PageHeader } from '@/components/ui/page-header'
import { NewCourseForm } from '@/features/courses/new-course-form'

export const Route = createFileRoute('/_shell/courses/new')({
  staticData: { crumb: '新建课程' },
  component: NewCoursePage,
})

function NewCoursePage() {
  return (
    <div className="space-y-5">
      <PageHeader title="新建课程" description="创建课程后即可导入课时素材。" />
      <Root defaultValue="blank" className="space-y-5">
        <List aria-label="课程创建方式" className="flex flex-wrap gap-1 border-b border-border">
          {[
            ['blank', '空白课程'],
            ['audio', '音频导入'],
            ['images', '图片 / 扫描 PDF'],
            ['pptx', 'PPTX 课件'],
            ['batch', '批量文件'],
          ].map(([value, label]) => (
            <Trigger
              key={value}
              value={value!}
              className="border-b-2 border-transparent px-3 py-2 text-sm text-text-muted hover:text-text data-[state=active]:border-accent data-[state=active]:font-medium data-[state=active]:text-accent-strong"
            >
              {label}
            </Trigger>
          ))}
        </List>
        <Content value="blank">
          <NewCourseForm />
        </Content>
        <Content value="audio">
          <AudioCourseForm />
        </Content>
        {(['images', 'pptx', 'batch'] as const).map((mode) => (
          <Content key={mode} value={mode}>
            <FileCourseForm mode={mode} />
          </Content>
        ))}
      </Root>
    </div>
  )
}
