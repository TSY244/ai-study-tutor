import { defineConfig } from 'orval'

// ORVAL_INPUT 由 scripts/generate-api.mjs 注入：指向展开 X-Request-ID
// 公共响应头之后的 OpenAPI 文件；ORVAL_SOURCE_LABEL 记录真实来源（用于文件头标注）。
// 正常来源是发布契约 api/openapi.yaml；契约未发布前临时使用设计源
// docs/web-rebuild/openapi.yaml（见 README 阻塞项）。
const input = process.env.ORVAL_INPUT
if (!input) {
  throw new Error('ORVAL_INPUT 未设置。请通过 npm run api:gen 执行生成。')
}
const sourceLabel = process.env.ORVAL_SOURCE_LABEL ?? input

const header = [
  'GENERATED FILE - DO NOT EDIT',
  `Source: ${sourceLabel}`,
  'Regenerate: npm run api:gen',
]

export default defineConfig({
  'ai-study-tutor-api': {
    input,
    output: {
      mode: 'tags-split',
      target: 'src/api/generated/endpoints',
      schemas: 'src/api/generated/model',
      client: 'fetch',
      clean: true,
      override: {
        header: () => header,
        // 自定义 fetch 实例：同源相对路径、X-Request-ID、Problem Details 解析。
        mutator: {
          path: './src/api/http.ts',
          name: 'apiFetch',
        },
        fetch: {
          // 生成函数直接返回解析后的 body；响应头（X-Request-ID 等）由
          // apiFetch 内部的诊断层捕获，错误对象携带 requestId。
          includeHttpResponseReturnType: false,
        },
      },
      mock: {
        generators: [
          {
            type: 'msw',
            delay: false,
            generateEachHttpStatus: true,
          },
        ],
      },
    },
    hooks: {
      // 生成物用项目 Prettier 统一格式化，保证 `npm run api:gen` 可重复且无 diff 噪音。
      afterAllFilesWrite: 'prettier --write --ignore-unknown ./src/api/generated',
    },
  },
})
