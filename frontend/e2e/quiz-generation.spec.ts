import { expect, test } from '@playwright/test'

import { COURSE_STATS_ID, JOB_FAILED_ID, LESSON_RANDOM_ID } from '../src/mocks/fixtures'

const QUIZZES_URL = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/quizzes`

test.describe('测验生成与失败重试', () => {
  for (const mode of ['regular', 'review'] as const) {
    test(`从课时测验页提交${mode === 'regular' ? '常规' : '复习'}测验，进入测验生成任务`, async ({
      page,
    }) => {
      await page.goto(QUIZZES_URL)

      await page.getByRole('button', { name: '生成测验' }).click()
      const dialog = page.getByRole('dialog', { name: '生成知识点测验' })
      await expect(dialog).toBeVisible()
      await dialog.getByLabel('测验类型').selectOption(mode)
      await dialog.getByLabel('题量（可选）').fill('7')
      await dialog.getByLabel('出题备注（可选）').fill('重点覆盖边界条件和应用题')
      const endpoint = `/api/v1/lessons/${LESSON_RANDOM_ID}/${mode === 'review' ? 'review-' : ''}quiz-generations`
      const responsePromise = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === endpoint && response.request().method() === 'POST',
      )
      await dialog.getByRole('button', { name: '开始生成' }).click()

      const response = await responsePromise
      expect(response.status()).toBe(202)
      expect(response.request().postDataJSON()).toMatchObject({
        question_count: 7,
        generation_notes: '重点覆盖边界条件和应用题',
      })
      const accepted = (await response.json()) as { job: { id: string } }
      await expect(page).toHaveURL(new RegExp(`/jobs/${accepted.job.id}$`))
      await expect(page.getByRole('heading', { name: '测验生成' })).toBeVisible()
      await expect(page.getByText('排队中').first()).toBeVisible()
      await expect(page.getByRole('button', { name: /任务中心/ })).toHaveAccessibleName(
        /任务进行中/,
      )
    })
  }

  test('失败任务可重试并跳转到新任务详情', async ({ page }) => {
    await page.goto(`/jobs/${JOB_FAILED_ID}`)

    await expect(page.getByRole('heading', { name: '测验生成' })).toBeVisible()
    await page.getByRole('button', { name: '重试任务' }).click()

    await expect(page).toHaveURL(/\/jobs\/[0-9a-f-]{36}$/)
    await expect(page).not.toHaveURL(new RegExp(`/jobs/${JOB_FAILED_ID}$`))
    await expect(page.getByRole('heading', { name: '测验生成' })).toBeVisible()
    await expect(page.getByText('排队中').first()).toBeVisible()
    await expect(page.getByText(JOB_FAILED_ID)).toBeVisible()
  })
})
