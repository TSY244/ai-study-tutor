import type {
  Attempt,
  ClaudeKey,
  Course,
  Document,
  Job,
  JobEvent,
  JobEventEventType,
  Lesson,
  Progress,
  Quiz,
  SystemStatus,
  Weakness,
} from '@/api/generated/model'

/**
 * 精心维护的开发/演示 fixtures：让 MSW 开发模式呈现可理解的产品状态。
 * 覆盖范围刻意保持小——全量端点由 orval 生成的 faker handlers 兜底；
 * 各业务阶段的场景 fixtures（resource_busy、autosave 冲突、grading_failed
 * 等）在 F2+ 按测试需要补充。
 *
 * F1 演示数据：2 门课程、4 个课时（覆盖 ready/processing/failed 状态）、
 * 3 个任务（进行中/失败/成功）、1 份测验与 1 个进行中的作答。
 * F4 补充：文档与版本（随机变量课时含 3 个笔记版本）、课时进度趋势、
 * 4 条弱点（覆盖待复习/已掌握与两门课程）。
 */

export const systemStatusOk: SystemStatus = {
  status: 'ok',
  version: '0.3.0-draft',
  maintenance: { enabled: false, reason: null },
  database: { status: 'ok', message: null },
  worker: { status: 'ok', message: null },
  claude: { status: 'unknown', message: '尚未执行连通性测试' },
  storage: { status: 'ok', message: null },
}

export const systemStatusDegraded: SystemStatus = {
  ...systemStatusOk,
  status: 'degraded',
  claude: { status: 'unavailable', message: 'Claude 暂时不可用，非 AI 能力不受影响' },
}

/** Claude 网关只维护安全视图；fixture 永远不包含 api_key。 */
export const demoClaudeKeys: ClaudeKey[] = [
  {
    id: 'ck_demo_primary',
    name: '主线路',
    masked_key: 'sk-ant-...7K2P',
    fingerprint: 'sha256:4f91a7c2d310',
    model: 'claude-sonnet-4-5',
    base_url: 'https://gw-primary.example',
    enabled: true,
    priority: 10,
    weight: 100,
    timeout_seconds: 300,
    cooldown_seconds: 300,
    max_consecutive_failures: 2,
    status: 'healthy',
    consecutive_failures: 0,
    last_error_code: null,
    last_success_at: '2026-07-25T09:40:00Z',
    last_failure_at: null,
    cooldown_until: null,
    last_latency_ms: 842,
    created_at: '2026-07-25T08:00:00Z',
    updated_at: '2026-07-25T09:40:00Z',
  },
  {
    id: 'ck_demo_backup',
    name: '备用线路',
    masked_key: 'sk-ant-...3M8Q',
    fingerprint: 'sha256:9b3078d14ace',
    model: 'claude-sonnet-4-5',
    base_url: '',
    enabled: false,
    priority: 20,
    weight: 50,
    timeout_seconds: 300,
    cooldown_seconds: 300,
    max_consecutive_failures: 2,
    status: 'disabled',
    consecutive_failures: 0,
    last_error_code: null,
    last_success_at: null,
    last_failure_at: null,
    cooldown_until: null,
    last_latency_ms: null,
    created_at: '2026-07-25T08:05:00Z',
    updated_at: '2026-07-25T08:05:00Z',
  },
]

export const COURSE_STATS_ID = '11111111-1111-4111-8111-111111111111'
export const COURSE_LINALG_ID = '22222222-2222-4222-8222-222222222222'

export const LESSON_RANDOM_ID = 'a0000000-0000-4000-8000-000000000001'
export const LESSON_BAYES_ID = 'a0000000-0000-4000-8000-000000000002'
export const LESSON_CONFIDENCE_ID = 'a0000000-0000-4000-8000-000000000003'
export const LESSON_VECTOR_ID = 'a0000000-0000-4000-8000-000000000004'

export const JOB_RUNNING_ID = 'b0000000-0000-4000-8000-000000000001'
export const JOB_FAILED_ID = 'b0000000-0000-4000-8000-000000000002'
export const JOB_SUCCEEDED_ID = 'b0000000-0000-4000-8000-000000000003'

export const QUIZ_DEMO_ID = 'c0000000-0000-4000-8000-000000000001'
export const QUIZ_OBJECTIVE_ID = 'c0000000-0000-4000-8000-000000000002'
export const ATTEMPT_DEMO_ID = 'd0000000-0000-4000-8000-000000000001'
export const ATTEMPT_FAILED_ID = 'd0000000-0000-4000-8000-000000000002'
export const ATTEMPT_FAILED_WITH_JOB_ID = 'd0000000-0000-4000-8000-000000000003'
export const JOB_GRADING_FAILED_ID = 'b0000000-0000-4000-8000-000000000004'

// 题目 ID（契约 format: uuid；weaknesses evidence 同步引用）
export const Q_DEMO_1_ID = 'c0000000-0000-4000-8000-000000000101'
export const Q_DEMO_2_ID = 'c0000000-0000-4000-8000-000000000102'
export const Q_OBJ_1_ID = 'c0000000-0000-4000-8000-000000000201'
export const Q_OBJ_2_ID = 'c0000000-0000-4000-8000-000000000202'
export const Q_OBJ_3_ID = 'c0000000-0000-4000-8000-000000000203'

export const demoCourses: Course[] = [
  {
    id: COURSE_STATS_ID,
    title: '统计学基础',
    description: '描述统计、概率分布与推断统计的核心概念。',
    archived: false,
    lesson_count: 3,
    version: 4,
    created_at: '2026-07-01T09:00:00Z',
    updated_at: '2026-07-17T14:20:00Z',
  },
  {
    id: COURSE_LINALG_ID,
    title: '线性代数',
    description: '向量空间、矩阵运算与特征值。',
    archived: false,
    lesson_count: 1,
    version: 1,
    created_at: '2026-07-15T08:30:00Z',
    updated_at: '2026-07-16T11:05:00Z',
  },
]

export const demoLessons: Lesson[] = [
  {
    id: LESSON_RANDOM_ID,
    course_id: COURSE_STATS_ID,
    ordinal: 1,
    title: '随机变量与分布',
    description: '离散与连续随机变量、期望与方差。',
    status: 'ready',
    active_job_id: null,
    documents: { raw_text: true, student_notes: true, teacher_outline: true },
    mastery: 0.62,
    version: 3,
    created_at: '2026-07-02T09:00:00Z',
    updated_at: '2026-07-16T10:00:00Z',
  },
  {
    id: LESSON_BAYES_ID,
    course_id: COURSE_STATS_ID,
    ordinal: 2,
    title: '贝叶斯定理',
    description: '条件概率、先验与后验。',
    status: 'processing',
    active_job_id: JOB_RUNNING_ID,
    documents: { raw_text: true, student_notes: false, teacher_outline: false },
    mastery: null,
    version: 1,
    created_at: '2026-07-17T13:00:00Z',
    updated_at: '2026-07-17T14:20:00Z',
  },
  {
    id: LESSON_CONFIDENCE_ID,
    course_id: COURSE_STATS_ID,
    ordinal: 3,
    title: '置信区间',
    description: '区间估计与样本量。',
    status: 'failed',
    active_job_id: null,
    documents: { raw_text: false, student_notes: false, teacher_outline: false },
    mastery: null,
    version: 1,
    created_at: '2026-07-17T13:30:00Z',
    updated_at: '2026-07-17T13:45:00Z',
  },
  {
    id: LESSON_VECTOR_ID,
    course_id: COURSE_LINALG_ID,
    ordinal: 1,
    title: '向量与线性组合',
    description: '向量运算、张成与线性无关。',
    status: 'ready',
    active_job_id: null,
    documents: { raw_text: true, student_notes: true, teacher_outline: true },
    mastery: 0.35,
    version: 2,
    created_at: '2026-07-15T08:40:00Z',
    updated_at: '2026-07-16T11:05:00Z',
  },
]

export const demoJobs: Job[] = [
  {
    id: JOB_RUNNING_ID,
    kind: 'lesson_import',
    aggregate_type: 'lesson',
    aggregate_id: LESSON_BAYES_ID,
    generation_family: 'lesson_generation',
    retry_of_job_id: null,
    status: 'running',
    stage: '生成学生笔记',
    progress: 45,
    message: null,
    attempt_count: 1,
    max_attempts: 3,
    retryable: false,
    cancel_requested: false,
    error: null,
    created_at: '2026-07-17T14:15:00Z',
    started_at: '2026-07-17T14:15:05Z',
    completed_at: null,
    updated_at: '2026-07-17T14:19:30Z',
  },
  {
    id: JOB_FAILED_ID,
    kind: 'quiz_generate',
    aggregate_type: 'lesson',
    aggregate_id: LESSON_CONFIDENCE_ID,
    generation_family: null,
    retry_of_job_id: null,
    status: 'failed',
    stage: '调用 Claude 生成题目',
    progress: 30,
    message: null,
    attempt_count: 3,
    max_attempts: 3,
    retryable: true,
    cancel_requested: false,
    error: {
      code: 'claude_unavailable',
      detail: 'Claude 服务暂时不可用，请稍后重试。',
      retryable: true,
    },
    created_at: '2026-07-17T13:31:00Z',
    started_at: '2026-07-17T13:31:02Z',
    completed_at: '2026-07-17T13:45:00Z',
    updated_at: '2026-07-17T13:45:00Z',
  },
  {
    id: JOB_SUCCEEDED_ID,
    kind: 'lesson_import',
    aggregate_type: 'lesson',
    aggregate_id: LESSON_RANDOM_ID,
    generation_family: 'lesson_generation',
    retry_of_job_id: null,
    status: 'succeeded',
    stage: '完成',
    progress: 100,
    message: '课时已生成',
    attempt_count: 1,
    max_attempts: 3,
    retryable: false,
    cancel_requested: false,
    error: null,
    created_at: '2026-07-16T09:50:00Z',
    started_at: '2026-07-16T09:50:02Z',
    completed_at: '2026-07-16T09:52:00Z',
    updated_at: '2026-07-16T09:52:00Z',
  },
  {
    id: JOB_GRADING_FAILED_ID,
    kind: 'attempt_grade',
    aggregate_type: 'attempt',
    aggregate_id: ATTEMPT_FAILED_WITH_JOB_ID,
    generation_family: null,
    retry_of_job_id: null,
    status: 'failed',
    stage: 'AI 批阅',
    progress: 60,
    message: null,
    attempt_count: 3,
    max_attempts: 3,
    retryable: true,
    cancel_requested: false,
    error: {
      code: 'claude_unavailable',
      detail: 'Claude 服务暂时不可用，请稍后重试。',
      retryable: true,
    },
    created_at: '2026-07-17T15:20:00Z',
    started_at: '2026-07-17T15:20:02Z',
    completed_at: '2026-07-17T15:24:00Z',
    updated_at: '2026-07-17T15:24:00Z',
  },
]

export const demoQuiz: Quiz = {
  id: QUIZ_DEMO_ID,
  lesson_id: LESSON_RANDOM_ID,
  title: '随机变量与分布 · 随堂测验',
  source: 'regular',
  status: 'ready',
  questions: [
    {
      id: Q_DEMO_1_ID,
      type: 'single_choice',
      stem: '下列哪个是离散型随机变量？',
      options: { A: '某地的日降雨量', B: '掷一枚骰子的点数', C: '成年男性的身高', D: '灯泡的寿命' },
      tested_concepts: ['random-variable'],
      difficulty: 'easy',
      cognitive_level: 'recall',
      score: 5,
    },
    {
      id: Q_DEMO_2_ID,
      type: 'short_answer',
      stem: '写出二项分布 B(n, p) 的期望与方差。',
      options: null,
      tested_concepts: ['binomial-distribution'],
      difficulty: 'medium',
      cognitive_level: 'explain',
      score: 10,
    },
  ],
  created_at: '2026-07-16T10:05:00Z',
}

export const demoAttempt: Attempt = {
  id: ATTEMPT_DEMO_ID,
  quiz_id: QUIZ_DEMO_ID,
  status: 'in_progress',
  version: 1,
  auto_score: null,
  final_score: null,
  max_score: 15,
  grading_job_id: null,
  grading_error_code: null,
  grading_error_detail: null,
  questions: demoQuiz.questions,
  answers: [],
  started_at: '2026-07-17T15:00:00Z',
}

/** 全客观题测验：提交走 200 AttemptSubmitCompleted（无 Job）分支。 */
export const demoObjectiveQuiz: Quiz = {
  id: QUIZ_OBJECTIVE_ID,
  lesson_id: LESSON_RANDOM_ID,
  title: '期望与方差 · 客观题自测',
  source: 'regular',
  status: 'ready',
  questions: [
    {
      id: Q_OBJ_1_ID,
      type: 'single_choice',
      stem: '设 X 为离散型随机变量，下列哪一条一定成立？',
      options: {
        A: 'E(X) 一定为正数',
        B: 'E(X) 是 X 所有取值按概率的加权平均',
        C: 'E(X) 一定等于最可能的取值',
        D: 'E(X) 只与最大值有关',
      },
      tested_concepts: ['expectation'],
      difficulty: 'easy',
      cognitive_level: 'recall',
      score: 5,
    },
    {
      id: Q_OBJ_2_ID,
      type: 'multiple_choice',
      stem: '下列哪些属于离散型分布？（多选）',
      options: { A: '二项分布', B: '泊松分布', C: '正态分布', D: '均匀分布' },
      tested_concepts: ['distributions'],
      difficulty: 'medium',
      cognitive_level: 'explain',
      score: 5,
    },
    {
      id: Q_OBJ_3_ID,
      type: 'true_false',
      stem: '任何随机变量的方差都大于或等于 0。',
      options: null,
      tested_concepts: ['variance'],
      difficulty: 'easy',
      cognitive_level: 'recall',
      score: 5,
    },
  ],
  created_at: '2026-07-16T11:00:00Z',
}

/**
 * 客观题标准答案：仅 MSW 服务端判分使用，绝不通过任何 API 响应返回
 * （学生可见 Question 不含 correct_answer；提交后由 feedback 下发）。
 * 取值约定：single_choice 为选项键；multiple_choice 为升序选项键数组；
 * true_false 为 boolean。
 */
export const QUIZ_ANSWER_KEYS: Record<string, Record<string, unknown>> = {
  [QUIZ_DEMO_ID]: { [Q_DEMO_1_ID]: 'B' },
  [QUIZ_OBJECTIVE_ID]: {
    [Q_OBJ_1_ID]: 'B',
    [Q_OBJ_2_ID]: ['A', 'B'],
    [Q_OBJ_3_ID]: true,
  },
}

/** 批阅失败且无关联 Job：走 POST /attempts/{id}/grading-jobs 入口。 */
export const demoAttemptFailed: Attempt = {
  id: ATTEMPT_FAILED_ID,
  quiz_id: QUIZ_DEMO_ID,
  status: 'grading_failed',
  version: 13,
  auto_score: 5,
  final_score: null,
  max_score: 15,
  grading_job_id: null,
  grading_error_code: 'claude_unavailable',
  grading_error_detail: 'Claude 服务暂时不可用，主观题尚未批阅。',
  questions: demoQuiz.questions,
  answers: [
    {
      question_id: Q_DEMO_1_ID,
      answer: { value: 'B' },
      answer_version: 1,
      awarded_score: 5,
      is_correct: true,
      feedback: null,
      updated_at: '2026-07-17T15:05:00Z',
    },
    {
      question_id: Q_DEMO_2_ID,
      answer: { value: 'E(X) = np，Var(X) = np(1-p)。' },
      answer_version: 2,
      awarded_score: null,
      is_correct: null,
      feedback: null,
      updated_at: '2026-07-17T15:08:00Z',
    },
  ],
  started_at: '2026-07-17T15:00:00Z',
  submitted_at: '2026-07-17T15:10:00Z',
}

/** 批阅失败且关联可重试 Job：走 Job Retry 入口。 */
export const demoAttemptFailedWithJob: Attempt = {
  ...demoAttemptFailed,
  id: ATTEMPT_FAILED_WITH_JOB_ID,
  grading_job_id: JOB_GRADING_FAILED_ID,
}

export const demoProgressByCourse: Record<string, Progress> = {
  [COURSE_STATS_ID]: {
    scope_type: 'course',
    scope_id: COURSE_STATS_ID,
    mastery: 0.62,
    formula_version: 'v1',
    completed_attempts: 4,
    active_weaknesses: 2,
    trend: [
      {
        recorded_at: '2026-07-10T10:00:00Z',
        mastery: 0.45,
        formula_version: 'v1',
        source_attempt_id: null,
      },
      {
        recorded_at: '2026-07-16T10:00:00Z',
        mastery: 0.62,
        formula_version: 'v1',
        source_attempt_id: null,
      },
    ],
  },
  [COURSE_LINALG_ID]: {
    scope_type: 'course',
    scope_id: COURSE_LINALG_ID,
    mastery: 0.35,
    formula_version: 'v1',
    completed_attempts: 1,
    active_weaknesses: 1,
    trend: [
      {
        recorded_at: '2026-07-16T11:00:00Z',
        mastery: 0.35,
        formula_version: 'v1',
        source_attempt_id: null,
      },
    ],
  },
}

// ---------------------------------------------------------------------------
// F4：文档（当前版本 + 历史版本）、课时进度、弱点
// ---------------------------------------------------------------------------

export const DOC_RANDOM_NOTES_V1_ID = 'f0000000-0000-4000-8000-0000000000a1'
export const DOC_RANDOM_NOTES_V2_ID = 'f0000000-0000-4000-8000-0000000000a2'
export const DOC_RANDOM_NOTES_V3_ID = 'f0000000-0000-4000-8000-0000000000a3'
export const DOC_RANDOM_RAW_ID = 'f0000000-0000-4000-8000-0000000000b1'
export const DOC_RANDOM_OUTLINE_ID = 'f0000000-0000-4000-8000-0000000000c1'
export const DOC_BAYES_RAW_ID = 'f0000000-0000-4000-8000-0000000000d1'
export const DOC_VECTOR_NOTES_ID = 'f0000000-0000-4000-8000-0000000000e1'

const RANDOM_NOTES_V3_MD = `# 随机变量与分布

> 本笔记由导入材料自动生成，用于课前预习与课后复习。

## 随机变量的定义

随机变量（random variable）是定义在样本空间 $\\Omega$ 上的实值函数 $X: \\Omega \\to \\mathbb{R}$。它把随机试验的结果映射为可以运算的数。

### 离散型随机变量

取值可以一一列举的随机变量称为离散型随机变量，例如掷一枚骰子的点数。

### 连续型随机变量

取值充满某个区间的随机变量称为连续型随机变量，例如灯泡的寿命。对连续型随机变量，$P(X = a) = 0$，概率由密度函数下的面积给出。

## 期望与方差

期望描述随机变量的平均水平，方差描述离散程度：

$$
E(X) = \\sum_{i} x_i \\, p_i, \\qquad \\mathrm{Var}(X) = E(X^2) - [E(X)]^2
$$

### 计算示例

\`\`\`python
import numpy as np

def expected_value(values, probs):
    """离散随机变量的期望。"""
    return float(np.dot(values, probs))

xs = [1, 2, 3, 4, 5, 6]
ps = [1 / 6] * 6
print(expected_value(xs, ps))  # 3.5
\`\`\`

## 常见分布速查

| 分布 | 期望 | 方差 |
| --- | --- | --- |
| 二项分布 $B(n, p)$ | $np$ | $np(1-p)$ |
| 泊松分布 $\\mathrm{Poisson}(\\lambda)$ | $\\lambda$ | $\\lambda$ |
| 正态分布 $N(\\mu, \\sigma^2)$ | $\\mu$ | $\\sigma^2$ |

## 常见误区

- 把条件概率 $P(A|B)$ 与联合概率 $P(AB)$ 混为一谈。
- 认为不相关就一定独立——只有联合正态等特例下成立。
- 方差计算时漏掉平方：$\\mathrm{Var}(X) = E(X^2) - [E(X)]^2$。

\`\`\`
错题本：P(A|B) = P(AB) / P(B)，分母是条件事件的概率。
\`\`\`
`

const RANDOM_NOTES_V2_MD = `# 随机变量与分布

## 随机变量的定义

随机变量是定义在样本空间 $\\Omega$ 上的实值函数 $X: \\Omega \\to \\mathbb{R}$。

### 离散型随机变量

取值可以一一列举的随机变量，例如掷骰子的点数。

## 期望与方差

$$
E(X) = \\sum_{i} x_i \\, p_i, \\qquad \\mathrm{Var}(X) = E(X^2) - [E(X)]^2
$$
`

const RANDOM_NOTES_V1_MD = `# 随机变量与分布

## 随机变量的定义

随机变量是样本空间上的实值函数。
`

const RANDOM_RAW_MD = `# 导入原文：随机变量与分布

本章介绍随机变量的概念。我们先定义离散型随机变量，再讨论连续型随机变量。

随机变量是定义在样本空间上的实值函数。常见的离散分布包括二项分布与泊松分布；常见的连续分布包括均匀分布与正态分布。

期望 $E(X)$ 度量平均水平，方差 $\\mathrm{Var}(X)$ 度量离散程度。请结合课后习题 3、5、7 练习计算。
`

const RANDOM_OUTLINE_MD = `# 随机变量与分布 · 教学大纲

## 教学目标

- 理解随机变量的定义与分类。
- 熟练计算常见分布的期望与方差。

## 教学重点

1. 离散型与连续型随机变量的区别。
2. 期望与方差的计算：$E(X)$ 与 $\\mathrm{Var}(X)$。

## 建议流程

1. 导入（5 分钟）：从掷骰子引入。
2. 概念讲解（20 分钟）：定义、分布列、密度函数。
3. 例题演练（15 分钟）：期望与方差计算。
4. 小结与作业布置（5 分钟）。
`

const BAYES_RAW_MD = `# 导入原文：贝叶斯定理

条件概率 $P(A|B) = P(AB) / P(B)$。贝叶斯定理给出先验与后验的关系：

$$
P(A|B) = \\frac{P(B|A) \\, P(A)}{P(B)}
$$

结合疾病筛查例题理解假阳性对后验概率的影响。
`

const VECTOR_NOTES_MD = `# 向量与线性组合

## 向量的运算

向量加法与数乘满足八条运算法则，构成线性空间。

## 线性组合与张成

向量 $\\mathbf{v}$ 是 $\\mathbf{v}_1, \\dots, \\mathbf{v}_k$ 的线性组合，当且仅当存在标量 $c_i$ 使 $\\mathbf{v} = \\sum_i c_i \\mathbf{v}_i$。所有线性组合构成的集合称为张成空间。
`

export const demoDocuments: Document[] = [
  {
    id: DOC_RANDOM_NOTES_V3_ID,
    lesson_id: LESSON_RANDOM_ID,
    kind: 'student_notes',
    revision: 3,
    content_md: RANDOM_NOTES_V3_MD,
    source_job_id: JOB_SUCCEEDED_ID,
    is_active: true,
    created_at: '2026-07-16T09:52:00Z',
  },
  {
    id: DOC_RANDOM_NOTES_V2_ID,
    lesson_id: LESSON_RANDOM_ID,
    kind: 'student_notes',
    revision: 2,
    content_md: RANDOM_NOTES_V2_MD,
    source_job_id: JOB_SUCCEEDED_ID,
    is_active: false,
    created_at: '2026-07-14T10:20:00Z',
  },
  {
    id: DOC_RANDOM_NOTES_V1_ID,
    lesson_id: LESSON_RANDOM_ID,
    kind: 'student_notes',
    revision: 1,
    content_md: RANDOM_NOTES_V1_MD,
    source_job_id: JOB_SUCCEEDED_ID,
    is_active: false,
    created_at: '2026-07-12T08:40:00Z',
  },
  {
    id: DOC_RANDOM_RAW_ID,
    lesson_id: LESSON_RANDOM_ID,
    kind: 'raw_text',
    revision: 1,
    content_md: RANDOM_RAW_MD,
    source_job_id: JOB_SUCCEEDED_ID,
    is_active: true,
    created_at: '2026-07-12T08:30:00Z',
  },
  {
    id: DOC_RANDOM_OUTLINE_ID,
    lesson_id: LESSON_RANDOM_ID,
    kind: 'teacher_outline',
    revision: 1,
    content_md: RANDOM_OUTLINE_MD,
    source_job_id: JOB_SUCCEEDED_ID,
    is_active: true,
    created_at: '2026-07-16T09:52:00Z',
  },
  {
    id: DOC_BAYES_RAW_ID,
    lesson_id: LESSON_BAYES_ID,
    kind: 'raw_text',
    revision: 1,
    content_md: BAYES_RAW_MD,
    source_job_id: JOB_RUNNING_ID,
    is_active: true,
    created_at: '2026-07-17T14:16:00Z',
  },
  {
    id: DOC_VECTOR_NOTES_ID,
    lesson_id: LESSON_VECTOR_ID,
    kind: 'student_notes',
    revision: 1,
    content_md: VECTOR_NOTES_MD,
    source_job_id: null,
    is_active: true,
    created_at: '2026-07-16T11:05:00Z',
  },
]

export const demoProgressByLesson: Record<string, Progress> = {
  [LESSON_RANDOM_ID]: {
    scope_type: 'lesson',
    scope_id: LESSON_RANDOM_ID,
    mastery: 0.62,
    formula_version: 'v1',
    completed_attempts: 3,
    active_weaknesses: 1,
    trend: [
      {
        recorded_at: '2026-07-12T10:00:00Z',
        mastery: 0.3,
        formula_version: 'v1',
        source_attempt_id: null,
      },
      {
        recorded_at: '2026-07-14T11:00:00Z',
        mastery: 0.48,
        formula_version: 'v1',
        source_attempt_id: null,
      },
      {
        recorded_at: '2026-07-16T10:00:00Z',
        mastery: 0.62,
        formula_version: 'v1',
        source_attempt_id: ATTEMPT_DEMO_ID,
      },
    ],
  },
  [LESSON_VECTOR_ID]: {
    scope_type: 'lesson',
    scope_id: LESSON_VECTOR_ID,
    mastery: 0.35,
    formula_version: 'v1',
    completed_attempts: 1,
    active_weaknesses: 1,
    trend: [
      {
        recorded_at: '2026-07-16T11:00:00Z',
        mastery: 0.35,
        formula_version: 'v1',
        source_attempt_id: null,
      },
    ],
  },
}

export const demoWeaknesses: Weakness[] = [
  {
    id: 'e0000000-0000-4000-8000-000000000001',
    course_id: COURSE_STATS_ID,
    lesson_id: LESSON_BAYES_ID,
    concept_key: 'conditional-probability',
    concept_label: '条件概率',
    description: '在 P(A|B) 与 P(AB) 之间反复混淆：把条件概率写成了联合概率，或分母用错事件。',
    evidence: [
      {
        attempt_id: ATTEMPT_DEMO_ID,
        question_id: Q_DEMO_2_ID,
        excerpt: '把 P(A|B) 写成 P(AB)，未除以 P(B)。',
      },
    ],
    severity: 4,
    review_count: 2,
    mastered: false,
    version: 3,
    first_seen_at: '2026-07-12T10:00:00Z',
    last_seen_at: '2026-07-17T15:10:00Z',
  },
  {
    id: 'e0000000-0000-4000-8000-000000000002',
    course_id: COURSE_STATS_ID,
    lesson_id: LESSON_RANDOM_ID,
    concept_key: 'variance-formula',
    concept_label: '方差计算',
    description: '使用 Var(X) = E(X²) − [E(X)]² 时漏掉平方项，符号错误率高。',
    evidence: [
      {
        attempt_id: ATTEMPT_DEMO_ID,
        question_id: Q_DEMO_2_ID,
        excerpt: 'E(X²) 写成了 [E(X)]²。',
      },
    ],
    severity: 3,
    review_count: 1,
    mastered: false,
    version: 1,
    first_seen_at: '2026-07-14T09:00:00Z',
    last_seen_at: '2026-07-16T10:30:00Z',
  },
  {
    id: 'e0000000-0000-4000-8000-000000000003',
    course_id: COURSE_STATS_ID,
    lesson_id: null,
    concept_key: 'expectation-linearity',
    concept_label: '期望的线性性',
    description: '曾误认 E(X+Y) 需要独立性；近期两次测验均已答对，标记为已掌握。',
    evidence: [],
    severity: 2,
    review_count: 3,
    mastered: true,
    version: 5,
    first_seen_at: '2026-07-10T08:00:00Z',
    last_seen_at: '2026-07-15T16:00:00Z',
  },
  {
    id: 'e0000000-0000-4000-8000-000000000004',
    course_id: COURSE_LINALG_ID,
    lesson_id: LESSON_VECTOR_ID,
    concept_key: 'linear-combination',
    concept_label: '线性组合与张成',
    description: '判断一个向量是否属于张成空间时，不会转化为线性方程组求解。',
    evidence: [
      {
        attempt_id: null,
        question_id: null,
        excerpt: '未能写出 c₁v₁ + c₂v₂ = v 的方程组形式。',
      },
    ],
    severity: 3,
    review_count: 0,
    mastered: false,
    version: 1,
    first_seen_at: '2026-07-16T11:20:00Z',
    last_seen_at: '2026-07-16T11:20:00Z',
  },
]

// ---------------------------------------------------------------------------
// F6：任务事件历史（REST 审计 JobEvent，任务详情时间线）。
// 注意与 SSE 的 JobStreamData 是两个 Schema；这里模拟数据库审计表，
// id 单调递增（不连续），列表最新在前。JOB_RUNNING_ID 合成 25+ 条
// 进度事件，供游标分页（加载更多）测试。
// ---------------------------------------------------------------------------

interface JobEventSeed {
  event_type: JobEventEventType
  stage: string
  progress: number
  message?: string | null
  occurred_at: string
}

function buildJobEvents(jobId: string, firstId: number, seeds: JobEventSeed[]): JobEvent[] {
  return seeds.map((seed, index) => ({
    id: firstId + index,
    job_id: jobId,
    event_type: seed.event_type,
    stage: seed.stage,
    progress: seed.progress,
    message: seed.message ?? null,
    occurred_at: seed.occurred_at,
  }))
}

/** 进行中任务：排队→开始→24 条进度（覆盖两页分页）。 */
const runningJobEvents = ((): JobEvent[] => {
  const seeds: JobEventSeed[] = [
    {
      event_type: 'job.queued',
      stage: '排队中',
      progress: 0,
      occurred_at: '2026-07-17T14:15:00Z',
    },
    {
      event_type: 'job.started',
      stage: '解析素材',
      progress: 2,
      occurred_at: '2026-07-17T14:15:05Z',
    },
  ]
  for (let i = 1; i <= 24; i += 1) {
    seeds.push({
      event_type: 'job.progress',
      stage: '生成学生笔记',
      progress: Math.min(2 + i * 2, 45),
      occurred_at: `2026-07-17T14:${String(15 + Math.floor(i / 2)).padStart(2, '0')}:${
        i % 2 === 0 ? '30' : '00'
      }Z`,
    })
  }
  return buildJobEvents(JOB_RUNNING_ID, 801, seeds)
})()

const failedJobEvents = buildJobEvents(JOB_FAILED_ID, 701, [
  {
    event_type: 'job.queued',
    stage: '排队中',
    progress: 0,
    occurred_at: '2026-07-17T13:31:00Z',
  },
  {
    event_type: 'job.started',
    stage: '准备上下文',
    progress: 5,
    occurred_at: '2026-07-17T13:31:02Z',
  },
  {
    event_type: 'job.progress',
    stage: '调用 Claude 生成题目',
    progress: 30,
    occurred_at: '2026-07-17T13:38:00Z',
  },
  {
    event_type: 'job.failed',
    stage: '调用 Claude 生成题目',
    progress: 30,
    message: 'Claude 服务暂时不可用，请稍后重试。',
    occurred_at: '2026-07-17T13:45:00Z',
  },
])

const succeededJobEvents = buildJobEvents(JOB_SUCCEEDED_ID, 601, [
  {
    event_type: 'job.queued',
    stage: '排队中',
    progress: 0,
    occurred_at: '2026-07-16T09:50:00Z',
  },
  {
    event_type: 'job.started',
    stage: '解析素材',
    progress: 3,
    occurred_at: '2026-07-16T09:50:02Z',
  },
  {
    event_type: 'job.progress',
    stage: '生成学生笔记',
    progress: 55,
    occurred_at: '2026-07-16T09:51:00Z',
  },
  {
    event_type: 'job.validating',
    stage: '校验生成结果',
    progress: 95,
    occurred_at: '2026-07-16T09:51:50Z',
  },
  {
    event_type: 'job.succeeded',
    stage: '完成',
    progress: 100,
    message: '课时已生成',
    occurred_at: '2026-07-16T09:52:00Z',
  },
])

const gradingFailedJobEvents = buildJobEvents(JOB_GRADING_FAILED_ID, 901, [
  {
    event_type: 'job.queued',
    stage: '排队中',
    progress: 0,
    occurred_at: '2026-07-17T10:00:00Z',
  },
  {
    event_type: 'job.started',
    stage: '客观题判分',
    progress: 20,
    occurred_at: '2026-07-17T10:00:03Z',
  },
  {
    event_type: 'job.progress',
    stage: 'AI 批阅',
    progress: 60,
    occurred_at: '2026-07-17T10:02:00Z',
  },
  {
    event_type: 'job.failed',
    stage: 'AI 批阅',
    progress: 60,
    message: 'Claude 服务暂时不可用，请稍后重试。',
    occurred_at: '2026-07-17T10:05:00Z',
  },
])

const demoJobEventsByJobId = new Map<string, JobEvent[]>([
  [JOB_RUNNING_ID, runningJobEvents],
  [JOB_FAILED_ID, failedJobEvents],
  [JOB_SUCCEEDED_ID, succeededJobEvents],
  [JOB_GRADING_FAILED_ID, gradingFailedJobEvents],
])

/** 任务事件历史：最新在前（审计倒序），未知任务返回空列表。 */
export function demoJobEvents(jobId: string): JobEvent[] {
  const events = demoJobEventsByJobId.get(jobId) ?? []
  return [...events].reverse()
}
