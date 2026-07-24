import { isApiProblemError, type KnownProblemCode } from '@/api/problem'

/**
 * Problem code -> 稳定前端呈现（02-frontend-handoff.md §7）。
 * 分支与文案只依赖 code，绝不解析服务端自然语言 detail；
 * detail 仅作为诊断信息展示（ErrorState），不参与行为决策。
 *
 * retryable 的含义：同一意图可用同一 Idempotency-Key 安全重放
 * （网络层失败、瞬态 5xx、限流）。确定性 4xx 标 false——重试前
 * 用户必须先改变输入或外部状态。
 */
export interface ProblemPresentation {
  title: string
  message: string
  retryable: boolean
}

// Record 以 KnownProblemCode 为键：契约新增码未映射时编译期报错，表即交接物。
const PROBLEM_PRESENTATIONS: Record<KnownProblemCode, ProblemPresentation> = {
  // —— F2 要求的核心映射 ——
  validation_error: {
    title: '表单未通过校验',
    message: '请按各字段下的提示修正后重新提交。',
    retryable: false,
  },
  version_conflict: {
    title: '版本冲突',
    message: '内容已被其他操作更新。请查看当前版本，合并你的修改后重试。',
    retryable: false,
  },
  resource_busy: {
    title: '已有任务在进行',
    message: '同一课时同时只允许一个生成任务。请查看现有任务，等待完成后再操作。',
    retryable: false,
  },
  idempotency_in_progress: {
    title: '相同请求正在处理',
    message: '你刚才的提交仍在处理中，请勿重复提交；稍后刷新查看结果。',
    retryable: true,
  },
  claude_not_configured: {
    title: 'AI 服务未配置',
    message: 'Worker 尚未配置 Claude Key。请在部署侧配置（浏览器不保存任何 Key）。',
    retryable: false,
  },
  claude_unavailable: {
    title: 'AI 服务暂不可用',
    message: '任务和你的输入已保留。请稍后重试，或先在系统状态页查看服务情况。',
    retryable: true,
  },
  job_not_retryable: {
    title: '任务不可重试',
    message: '该任务当前状态不支持重试。请按任务详情中的原因选择后续操作。',
    retryable: false,
  },
  storage_full: {
    title: '存储空间不足',
    message: '已阻止新的导入。请清理不需要的数据或更换数据目录后重试。',
    retryable: false,
  },
  upload_too_large: {
    title: '文件超过大小限制',
    message: '请选择更小的文件，或将内容拆分后分批导入。',
    retryable: false,
  },
  network_error: {
    title: '网络连接失败',
    message: '无法连接本地服务。你的输入已保留，确认服务运行后重试。',
    retryable: true,
  },
  // —— 其余契约码：同样按 code 给稳定文案 ——
  bad_request: {
    title: '请求不合法',
    message: '请求参数不符合接口要求。请刷新页面后重试。',
    retryable: false,
  },
  not_found: {
    title: '资源不存在',
    message: '内容不存在或已被删除。请返回上一级资源继续。',
    retryable: false,
  },
  idempotency_conflict: {
    title: '请求标识冲突',
    message: '同一请求标识携带了不同内容。请刷新页面后重新操作。',
    retryable: false,
  },
  attempt_already_submitted: {
    title: '答卷已提交',
    message: '本次作答已经提交，不能再修改。请查看批阅结果。',
    retryable: false,
  },
  practice_not_needed: {
    title: '暂不需要复习测验',
    message: '当前没有足够依据生成复习测验。请先完成一次普通测验。',
    retryable: false,
  },
  unsupported_media_type: {
    title: '文件类型不支持',
    message: '请选择支持的文件格式（如 PDF、Word、Markdown、音频）后重试。',
    retryable: false,
  },
  too_many_requests: {
    title: '请求过于频繁',
    message: '请稍等片刻后重试。',
    retryable: true,
  },
  maintenance_mode: {
    title: '系统维护中',
    message: '本地服务正在维护（如备份恢复）。你的数据未丢失，请稍后重试。',
    retryable: true,
  },
  http_error: {
    title: '服务响应异常',
    message: '本地服务返回了无法识别的错误。请稍后重试；若持续出现请重启服务。',
    retryable: true,
  },
  unknown_error: {
    title: '未知错误',
    message: '发生了未预期的错误。请重试；若持续出现请记录请求 ID 排查。',
    retryable: false,
  },
}

const FALLBACK: ProblemPresentation = PROBLEM_PRESENTATIONS.unknown_error

/** 按 code 取稳定呈现；契约新增码未映射时兜底为 unknown_error（不崩溃）。 */
export function presentProblemCode(code: string): ProblemPresentation {
  return code in PROBLEM_PRESENTATIONS ? PROBLEM_PRESENTATIONS[code as KnownProblemCode] : FALLBACK
}

/**
 * 从任意错误取呈现：非 Problem 错误（如组件异常）按 unknown_error 处理。
 * 调用方需要 requestId / extensions 时仍从 ApiProblemError 本体取。
 */
export function presentProblem(error: unknown): ProblemPresentation {
  return presentProblemCode(isApiProblemError(error) ? error.code : 'unknown_error')
}

/** resource_busy 的契约扩展（04-api-contract.md §3）：用于关联现有任务。 */
export interface ResourceBusyInfo {
  activeJobId?: string
  generationFamily?: string
}

export function getResourceBusyInfo(error: unknown): ResourceBusyInfo | null {
  if (!isApiProblemError(error) || error.code !== 'resource_busy') return null
  return {
    activeJobId: error.getExtension<string>('active_job_id'),
    generationFamily: error.getExtension<string>('generation_family'),
  }
}
