import { expect, test } from '@playwright/test'

import { COURSE_STATS_ID, JOB_FAILED_ID, LESSON_RANDOM_ID } from '../src/mocks/fixtures'

const QUIZZES_URL = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/quizzes`

test.describe('测验生成与失败重试', () => {
  test('从课时测验页发起生成，进入测验生成任务', async ({ page }) => {
    await page.goto(QUIZZES_URL)

    await page.getByRole('button', { name: '生成测验' }).click()

    await expect(page).toHaveURL(/\/jobs\/[0-9a-f-]{36}$/)
    await expect(page.getByRole('heading', { name: '测验生成' })).toBeVisible()
    await expect(page.getByText('排队中').first()).toBeVisible()
    await expect(page.getByRole('button', { name: /任务中心/ })).toHaveAccessibleName(/任务进行中/)
  })

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
