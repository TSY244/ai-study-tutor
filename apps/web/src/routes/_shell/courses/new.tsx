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
      <NewCourseForm />
    </div>
  )
}
