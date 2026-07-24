#!/usr/bin/env node
/**
 * OpenAPI Client 生成入口。
 *
 * 输入解析顺序：
 *   1. --source=<path> 命令行参数（相对仓库根目录或绝对路径）
 *   2. 环境变量 API_CONTRACT_PATH
 *   3. 发布契约 api/openapi.yaml
 *   4. 回退：设计源 docs/web-rebuild/openapi.yaml（草案，报告中标记阻塞）
 *
 * 无论来源如何，都会先确定性地把顶层 x-common-response-headers 展开到每一个
 * 成功与错误 Response（发布契约已展开时为幂等操作），再调用 orval 生成
 * TypeScript 类型、fetch Client 与 MSW handlers 到 src/api/generated/。
 */
import { spawnSync } from 'node:child_process'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { parse, stringify } from 'yaml'

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const repoRoot = path.resolve(webRoot, '..', '..')

function resolveSource() {
  const arg = process.argv.find((a) => a.startsWith('--source='))
  const explicit = arg?.slice('--source='.length) || process.env.API_CONTRACT_PATH
  if (explicit) {
    const abs = path.isAbsolute(explicit) ? explicit : path.resolve(repoRoot, explicit)
    if (!existsSync(abs)) {
      console.error(`[api:gen] 指定的契约文件不存在: ${abs}`)
      process.exit(1)
    }
    return { path: abs, kind: 'explicit' }
  }
  const published = path.join(repoRoot, 'api', 'openapi.yaml')
  if (existsSync(published)) return { path: published, kind: 'published' }
  const draft = path.join(repoRoot, 'docs', 'web-rebuild', 'openapi.yaml')
  if (existsSync(draft)) return { path: draft, kind: 'design-draft' }
  console.error('[api:gen] 找不到 api/openapi.yaml，也找不到 docs/web-rebuild/openapi.yaml。')
  process.exit(1)
}

/** 把 x-common-response-headers 展开到所有 Response 对象（幂等）。返回展开数量。 */
function expandCommonResponseHeaders(doc) {
  const common = doc['x-common-response-headers']
  if (!common || typeof common !== 'object') return 0
  let expanded = 0

  const apply = (response) => {
    if (!response || typeof response !== 'object' || '$ref' in response) return
    response.headers ??= {}
    for (const [name, value] of Object.entries(common)) {
      if (!(name in response.headers)) {
        response.headers[name] = value
        expanded += 1
      }
    }
  }

  // components.responses 中的共享响应
  for (const response of Object.values(doc.components?.responses ?? {})) apply(response)
  // 路径内联响应（$ref 指向共享响应的已由上一步覆盖）
  for (const pathItem of Object.values(doc.paths ?? {})) {
    if (!pathItem || typeof pathItem !== 'object') continue
    for (const operation of Object.values(pathItem)) {
      if (!operation || typeof operation !== 'object' || !('responses' in operation)) continue
      for (const response of Object.values(operation.responses ?? {})) apply(response)
    }
  }
  return expanded
}

/**
 * Orval 的 fetch client 会把 text/event-stream 当成一次性 Promise 响应。
 * REST 生成输入排除这类操作；SSE 由 src/api/job-stream.ts 的增量解析器负责。
 */
function excludeStreamingOperations(doc) {
  const methods = new Set(['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'])
  let excluded = 0
  for (const pathItem of Object.values(doc.paths ?? {})) {
    if (!pathItem || typeof pathItem !== 'object') continue
    for (const [method, operation] of Object.entries(pathItem)) {
      if (!methods.has(method) || !operation || typeof operation !== 'object') continue
      const hasEventStream = Object.values(operation.responses ?? {}).some((response) => {
        if (!response || typeof response !== 'object' || '$ref' in response) return false
        return 'text/event-stream' in (response.content ?? {})
      })
      if (hasEventStream) {
        delete pathItem[method]
        excluded += 1
      }
    }
  }
  return excluded
}

const source = resolveSource()
if (source.kind === 'design-draft') {
  console.warn(
    '[api:gen] 警告：发布契约 api/openapi.yaml 不存在，正在使用设计源\n' +
      `          ${path.relative(repoRoot, source.path)} 生成临时 Client。\n` +
      '          契约发布后请重新运行 npm run api:gen，并核对 breaking changes。',
  )
}

const doc = parse(readFileSync(source.path, 'utf8'))
const expandedCount = expandCommonResponseHeaders(doc)
const excludedStreams = excludeStreamingOperations(doc)

const cacheDir = path.join(webRoot, '.openapi-cache')
mkdirSync(cacheDir, { recursive: true })
const expandedPath = path.join(cacheDir, 'openapi.expanded.yaml')
writeFileSync(expandedPath, stringify(doc), 'utf8')
console.log(
  `[api:gen] 来源: ${path.relative(repoRoot, source.path)} (${source.kind})；` +
    `展开 ${expandedCount} 个 X-Request-ID 响应头，排除 ${excludedStreams} 个 SSE 操作` +
    ` -> ${path.relative(webRoot, expandedPath)}`,
)

const result = spawnSync('npx', ['orval', '--config', 'orval.config.ts'], {
  cwd: webRoot,
  stdio: 'inherit',
  env: {
    ...process.env,
    ORVAL_INPUT: expandedPath,
    ORVAL_SOURCE_LABEL:
      source.kind === 'published'
        ? 'api/openapi.yaml (发布契约)'
        : source.kind === 'explicit'
          ? `${path.relative(repoRoot, source.path)} (显式指定)`
          : 'docs/web-rebuild/openapi.yaml (设计草案，等待发布契约 api/openapi.yaml)',
  },
})
if (result.status !== 0) {
  console.error(`[api:gen] orval 生成失败（exit ${result.status}）。`)
  process.exit(result.status ?? 1)
}
console.log('[api:gen] 生成完成 -> src/api/generated/（生成物不可手改）')
