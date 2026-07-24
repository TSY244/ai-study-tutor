import { describe, expect, it } from 'vitest'

import { assertNoSensitiveEnv, findSensitiveEnvNames } from '@/env'

describe('敏感环境变量防护', () => {
  it('放行非敏感 VITE_ 配置', () => {
    expect(findSensitiveEnvNames({ VITE_APP_TITLE: 'x', VITE_ENABLE_MSW: 'true' })).toEqual([])
  })

  it('拒绝疑似敏感的 VITE_ 变量名', () => {
    expect(
      findSensitiveEnvNames({
        VITE_ANTHROPIC_API_KEY: 'x',
        VITE_DB_PASSWORD: 'x',
        VITE_CLAUDE_TOKEN: 'x',
        VITE_APP_SECRET: 'x',
      }),
    ).toEqual([
      'VITE_ANTHROPIC_API_KEY',
      'VITE_DB_PASSWORD',
      'VITE_CLAUDE_TOKEN',
      'VITE_APP_SECRET',
    ])
  })

  it('非 VITE_ 变量不参与前端扫描（服务端配置不进 bundle）', () => {
    expect(findSensitiveEnvNames({ API_PROXY_TARGET: 'http://127.0.0.1:8080' })).toEqual([])
  })

  it('assertNoSensitiveEnv 命中时抛出', () => {
    expect(() => assertNoSensitiveEnv({ VITE_API_KEY: 'x' })).toThrow(/VITE_API_KEY/)
    expect(() => assertNoSensitiveEnv({ VITE_APP_TITLE: 'x' })).not.toThrow()
  })
})
