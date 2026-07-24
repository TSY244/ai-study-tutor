/**
 * 前端环境变量：只允许 VITE_ 前缀的非敏感配置。
 * Claude Key / 数据库密码 / 服务端 secret 绝不允许出现在前端。
 * 启动时扫描所有 VITE_ 变量名，命中敏感命名直接拒绝启动。
 */

const SENSITIVE_NAME_PATTERN = /(KEY|SECRET|TOKEN|PASSWORD|CREDENTIAL)/i

export function findSensitiveEnvNames(env: Record<string, unknown>): string[] {
  return Object.keys(env).filter(
    (key) => key.startsWith('VITE_') && SENSITIVE_NAME_PATTERN.test(key),
  )
}

export function assertNoSensitiveEnv(env: Record<string, unknown>): void {
  const offenders = findSensitiveEnvNames(env)
  if (offenders.length > 0) {
    throw new Error(
      `检测到疑似敏感的前端环境变量：${offenders.join(', ')}。` +
        '前端只允许 VITE_ 前缀的非敏感配置；API Key 只能存在于 Worker Docker secret。',
    )
  }
}

assertNoSensitiveEnv(import.meta.env)

export const env = {
  appTitle: import.meta.env.VITE_APP_TITLE ?? 'AI 学习助教',
} as const
