import { gzipSync } from 'node:zlib'
import { readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'

import { chromium } from '@playwright/test'

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://127.0.0.1:8080'
const distRoot = fileURLToPath(new URL('../dist/', import.meta.url))
const requestedAssets = new Set()

const browser = await chromium.launch()
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  page.on('response', (response) => {
    const url = new URL(response.url())
    if (url.origin === new URL(baseURL).origin && url.pathname.startsWith('/assets/')) {
      requestedAssets.add(url.pathname)
    }
  })

  await page.goto(new URL('/dashboard', baseURL).href)
  await page.getByRole('heading', { name: '学习总览' }).waitFor()
  await page.waitForFunction(
    () => globalThis.document.querySelectorAll('[aria-busy="true"]').length === 0,
  )

  let jsGzipBytes = 0
  let cssGzipBytes = 0
  const assets = [...requestedAssets].sort()
  for (const asset of assets) {
    const contents = await readFile(`${distRoot}${asset.replace('/assets/', 'assets/')}`)
    const gzipBytes = gzipSync(contents, { level: 9 }).byteLength
    if (asset.endsWith('.js')) jsGzipBytes += gzipBytes
    if (asset.endsWith('.css')) cssGzipBytes += gzipBytes
  }

  const unexpectedLazyAssets = assets.filter((asset) =>
    /markdown-reader|katex|course-form|attempt-taking/i.test(asset),
  )
  const result = {
    route: '/dashboard',
    assetRequests: assets.length,
    jsRequests: assets.filter((asset) => asset.endsWith('.js')).length,
    initialJsGzipKiB: Number((jsGzipBytes / 1024).toFixed(2)),
    initialCssGzipKiB: Number((cssGzipBytes / 1024).toFixed(2)),
    unexpectedLazyAssets,
  }

  console.log(JSON.stringify(result, null, 2))
  if (unexpectedLazyAssets.length > 0) process.exitCode = 1
} finally {
  await browser.close()
}
