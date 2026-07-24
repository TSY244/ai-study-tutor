/**
 * 上传限制：契约只定义了 413 upload_too_large（04-api-contract.md §3），
 * 未枚举数值。前端预检用本地常量给出即时反馈；服务端 413 永远是最终
 * 事实来源。数值与后端配置的对齐是已知待定项（见阶段报告）。
 */
export const MAX_UPLOAD_BYTES = 100 * 1024 * 1024

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}
