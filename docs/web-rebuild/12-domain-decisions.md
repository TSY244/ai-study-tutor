# 领域决策记录

本文记录各后端阶段在冻结 legacy 行为基线与落地设计时作出的显式决策。
golden 夹具（`backend/testdata/legacy-fixtures/`）如实冻结 legacy 现状；本文是
目标系统与基线之间所有**有意差异**的唯一清单，每条都有对应测试断言。
未列入本文的行为默认与 golden 一致。

## B4：确定性学习领域

## D1 多选题按规范化集合判分（有意偏离 legacy）

- legacy：多选按规范化字符串逐项比较，顺序或重复不同即判错（golden 用例
  `multiple_choice_reordered_is_wrong_legacy_quirk`、`multiple_choice_duplicate_is_wrong_legacy_quirk`）。
- 目标系统：规范化、去重后按集合比较（03 §10.2；06 阶段 5 关键不变量
  “多选题比较规范化集合，不能受顺序影响”）。
- 兼容影响：旧数据中因乱序/重复被判错的多选作答不会改判（历史 Attempt 的
  判定已持久化）；新作答按集合判分。Go golden 测试对 quirk 用例显式登记
  目标期望值（`backend/internal/attempt/grading_golden_test.go` 的 `quirkTarget`），
  不是悄悄“修正” golden。

## D2 低掌握度迁移题约束冲突的解决（03 §10.1 v2）

- 冲突：legacy 低掌握度认知分布（transfer=0）与提示词中“至少 20% 迁移题”
  硬约束矛盾。
- 决策：分布表保持 golden 冻结不变；迁移题在低掌握度档降级为**建议**而非
  硬约束（中/高掌握档维持分布表达成的比例）。提示词措辞在 B6 实现时按
  v2 规则生成，不再出现与分布表冲突的硬约束。
- 兼容影响：仅影响 B6 提示词文本；不影响任何持久化数据。

## D3 题量显式覆盖短路提前停止

- legacy `determine_question_count`：`user_override` 非空时直接夹取到
  [5,15]，不评估提前停止（golden `override_beats_early_stop`）。
- 目标系统：`POST /lessons/{id}/quiz-generations` 携带显式 `question_count`
  时跳过策略门禁；为空时才评估（掌握度 ≥0.85 且连续正确 ≥3 →
  `422 practice_not_needed`，不创建 Job）。

## D4 薄弱点证据改为 JSONB 数组

- legacy：evidence 为换行拼接字符串。
- 目标系统：evidence 为 JSONB 数组，元素
  `{attempt_id, question_id, quiz_id, note, recorded_at}`。
  note 是脱敏摘要（如“客观题作答判分”），不含学习材料正文或完整答案。
- 重复消费防护：第一层是 lesson 范围掌握度快照的消费标记（同事务写入，
  `uq_mastery_snapshots_source` 兜底）；第二层是证据按 attempt+question
  去重。重复消费不增加 review_count、不追加证据、不新增快照。

## D5 severity 规范化与生命周期规则（保持 legacy）

- `severity <= 0`（含缺省）视为 3，上限 5（legacy `int(x or 3)` 冻结，
  golden `severity_zero_becomes_default_3_legacy_quirk`）。
- 合并：新旧 severity 取大、review_count+1、mastered 复位（fresh 错误证据
  重新打开薄弱点）。
- 复习：答对时 severity-1（下限 1）、review_count+1；severity 降到 1 且
  review_count >= 2 时标记已掌握。
- resolve（手动）：mastered=true 且 severity 归 1；reopen：mastered=false，
  severity 不变。两者都追加快照（source_attempt_id 为空）。
- 同一 Attempt 内同一概念先错后对：按题目顺序先合并错误证据、再应用复习
  （与 legacy 更新顺序一致）。

## D6 掌握度公式版本 mastery/v1 与量纲

- 公式：`0.50*成绩加权 + 0.30*薄弱点解决率 + 0.20*严重度因子`，无薄弱点时
  只用成绩（golden `chapter_mastery` 全量用例）。成绩加权为线性递增权重
  （weight = 1 + i*0.5，i 为按时间升序下标，被跳过的零分母记录也占下标）。
- 版本号 `mastery/v1`；公式变更必须引入新版本并保留历史快照。
- 存储：快照 `value` 为 0-100 百分比（NUMERIC(5,2)）；API（Progress 与
  Lesson.mastery）输出 0..1（读取时 /100）。
- lesson 范围掌握度包含课程级（lesson_id IS NULL）薄弱点（legacy
  “chapter None 计入每章”语义）。

## D7 反馈可见性矩阵

| Attempt 状态 | is_correct/awarded_score | correct_answer/explanation（feedback） |
| --- | --- | --- |
| in_progress | 不返回 | 不返回 |
| submitted/grading/grading_failed | 客观题可见 | 不返回 |
| graded | 全部可见 | 返回（QuestionFeedback） |

- 学生试卷视图（getQuiz/createAttempt/getAttempt.questions）永不返回
  `correct_answer`/`explanation`；它们只经 Attempt 反馈在 graded 后出现。
- 未作答：`is_correct=null`、awarded 0，与答错（false）语义不同；未作答
  不产生薄弱点证据、不计复习。
- 主观题未批阅前 `final_score=null`（不得显示伪造总分）。

## D8 状态码与错误映射决策（契约未逐条指定的部分）

| 场景 | 响应 |
| --- | --- |
| draft/archived 试卷开始作答 | 422 validation_error（试卷不可作答） |
| PATCH 保存不属于本试卷的题目 | 404 not_found |
| submit 全局 version 不一致 | 409 version_conflict + `current_version` 扩展 |
| PATCH answer_version 冲突 | 409 version_conflict + `current_answer_version`/`current_attempt_version`/`current_answer` 扩展 |
| grading-jobs 对 in_progress | 422 validation_error |
| grading-jobs 对 grading 且有活动批阅 Job | 409 resource_busy + `active_job_id` 扩展 |
| grading-jobs 对 submitted/graded 或 max_score 为空（旧隔离数据） | 409 attempt_already_submitted / 422 validation_error，不创建 Job |
| Job Retry 对活动状态任务 | 409 resource_busy |
| Job Retry 对 succeeded/cancelled | 422 validation_error |

## D9 幂等记录响应头回放

- `idempotency_records` 新增 `response_headers JSONB`：创建类端点（课程、
  课时、素材、Attempt）回放时连同 Location 头一起返回。
- 响应体存 JSONB，回放为语义等价 JSON（键序/空白由 jsonb 规范化），
  客户端不得做字节级比较。

## B5：Worker 素材管线

### D10 raw_text 即标准化文本；chunk 不落库

- lesson_import 在 B5 跳过 `generating_teacher_outline/
  generating_student_notes`（需 Claude，B6 接入）；发布的 raw_text 文档
  内容是经 `norm/v1` 规范化的完整文本（03 §8.3“保存 raw document
  版本”）。
- `chunk/v1` 分块结果不入库：Job result 只存 `chunk_count` 与
  `chunks_sha256` 清单聚合 hash。B6 生成任务以同一算法版本从 active
  raw_text 文档重新分块，并用该 hash 校验复现；算法变更必须引入新
  版本号。
- 兼容影响：无持久化 chunk 表，B6 依赖 `chunk/v1` 的确定性。

### D11 导入失败/取消的业务侧写规则

- 终态 `failed`：素材 `failed`（metadata 记录 `error_code`）、课时
  `failed`；自动重试 requeue 时不做任何侧写。
- 终态 `cancelled`：素材保持 `pending`（未处理，不污名化）；课时按
  是否已有发布文档归位（有 → `ready`，无 → `draft`）。
- 失败/取消不改变任何 active 文档（发布为单事务，崩溃整体回滚）。

### D12 URL 素材的二次 SSRF 校验

> **已被 Claude Web Fetch 路线取代（2026-07）**：URL 素材不再由 Worker
> 本地预检/下载；Worker 只做 canonicalClaudeURL 规范化与白名单校验，
> 本地 DNS 重绑定 / TOCTOU 风险随之消除。2026-07 晚些时候实际抓取
> 再从 Web Fetch server tool 改为 Worker 容器内 Claude Code CLI 的
> WebFetch 客户端工具（中转网关不转发 server tools；Worker 需出网访问
> 白名单站点）。以下为历史决策原文，保留备查。

- API 创建时校验一次；Worker 下载前再次校验（DNS 重绑定防护）：
  仅 https、host 白名单、DNS 全公网；预检请求跟随重定向逐跳重复
  校验，最多 5 跳，不下载正文。
- 已知残余风险：预检与 yt-dlp 实际下载之间存在 TOCTOU 窗口，且
  yt-dlp 进程侧不做网络隔离（见 13 §8；B8 可用容器网络策略补强）。

### D13 文本清洗的 legacy 一致性

- `norm/v1` 与 legacy `clean_text` 逐字节一致（含“不做行内 strip”
  的现状，golden `mixed_with_trim` 冻结）。
- `_clean_lrc` 不做整体规范化（legacy 现状）；统一由 normalizing
  阶段兜底。
- HTML 不可见容器移除是近似实现（RE2 无反向引用，对
  div/span/section 逐个匹配）。

### D14 未注册 Job kind 的处置

- registry miss → 终态 `failed(unsupported_job_kind)`（不排队空等）。
- AI 任务（B6）与导出（B7）注册前，相关 API 创建的 Job 会按此
  失败；各阶段注册后恢复正常。
