/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { visualizer } from 'rollup-plugin-visualizer'
import { defineConfig, loadEnv } from 'vite'

const INITIAL_PRELOAD_EXCLUDE = /(^|\/)(forms|katex|markdown)-[^/]+\.js$/

// 后端地址只用于本机 Vite dev server 的同源代理，绝不进入前端 bundle。
// 浏览器始终通过相对路径访问 /api、/healthz、/readyz（生产由 Caddy 同源反代）。
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const proxyTarget = env.API_PROXY_TARGET || 'http://127.0.0.1:8080'

  return {
    plugins: [
      // 单元测试（vitest）不需要路由代码生成/转换，避免其注入代码干扰组件测试。
      ...(mode === 'test'
        ? []
        : [
            tanstackRouter({
              target: 'react',
              autoCodeSplitting: true,
              routeFileIgnorePattern: '\\.(test|spec)\\.',
            }),
          ]),
      react(),
      tailwindcss(),
      // 每次生产构建输出 dist/bundle-report.html（gzip/brotli 体积， treemap）。
      ...(mode === 'test'
        ? []
        : [
            visualizer({
              filename: 'dist/bundle-report.html',
              template: 'treemap',
              gzipSize: true,
              brotliSize: true,
            }),
          ]),
    ],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      host: '127.0.0.1',
      proxy: {
        '/api': proxyTarget,
        '/healthz': proxyTarget,
        '/readyz': proxyTarget,
      },
    },
    build: {
      // 现代浏览器基线（Vite 8 默认），显式声明以便审阅。
      target: 'baseline-widely-available',
      modulePreload: {
        resolveDependencies(_filename, deps, context) {
          if (context.hostType !== 'html') return deps
          return deps.filter((dep) => !INITIAL_PRELOAD_EXCLUDE.test(dep))
        },
      },
      // 生成独立 .map 文件但不在 bundle 中引用：可本地诊断，不向用户暴露映射。
      sourcemap: 'hidden',
      rollupOptions: {
        output: {
          // Keep manual splitting narrow. Broader vendor groups can pull form/Markdown
          // dependencies into the app shell through shared runtime edges.
          manualChunks(id) {
            if (!id.includes('node_modules')) return undefined
            if (/(^|\/)node_modules\/(react|react-dom|scheduler)(\/|$)/.test(id)) {
              return 'react-vendor'
            }
            return undefined
          },
        },
      },
    },
    test: {
      environment: 'jsdom',
      setupFiles: ['./src/test/setup.ts'],
      include: ['src/**/*.{test,spec}.{ts,tsx}'],
      restoreMocks: true,
      unstubGlobals: true,
      // Docker 构建环境 CPU 受限，重型集成用例（整页渲染 + MSW）容易超时报抖动。
      testTimeout: 15000,
      coverage: {
        provider: 'v8',
        reporter: ['text', 'html'],
        include: ['src/**/*.{ts,tsx}'],
        exclude: ['src/api/generated/**', 'src/test/**', 'src/**/*.d.ts'],
      },
    },
  }
})
