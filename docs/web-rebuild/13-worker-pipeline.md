# Worker 素材管线（B5 交接物）

本文是 B5 阶段的稳定交接物：Worker Runner 编排、lesson_import 管线、
Job kind/stage 表、素材错误码、外部工具契约与演示数据。架构规则以
`03-backend-architecture.md` §7/§8 为准；本文记录**已实现**的行为。

## 1. Runner 编排（internal/worker）

- `Runner.Run` 常驻循环 / `RunOnce` 单次领取（测试与一次性运行用）。
- 领取：`job.Repository.Claim`（Business Gate shared lock + `FOR UPDATE
  SKIP LOCKED` + 租约），同一 Job 不会被两个 Worker 重复领取。
- 执行：每个 Job 有总时限 `WORKER_JOB_TIMEOUT`（默认 30m）；按
  `job_id` 隔离的临时目录 `APP_DATA_DIR/tmp/{job_id}/`，终态后由
  Runner 清理。
- 租约守护：心跳每 `Lease/3` 续租；轮询 `cancel_requested`（默认 2s）。
  租约丢失或取消请求都会取消任务上下文（子进程随之被 Kill）。
- 终态事务：
  - 成功：`Handler.Outcome.Publish(tx)`（领域发布）与 `repo.Complete`
    **同一事务**；
  - 失败：`repo.Fail` 内部决定 requeue（指数退避）或终态 failed；
    仅终态时同事务运行 `Handler.FinalizeFail`；
  - 取消：`repo.CancelByOwner` + `Handler.FinalizeCancel` 同一事务；
  - 租约丢失/进程关停：不写任务状态，由周期 Recovery 按
    `worker_lost` 回收重试。
- Handler registry 按 `jobs.kind` 分发；未注册 kind 直接终态
  `failed(unsupported_job_kind)`（AI 任务在 B6、导出在 B7 注册）。

## 2. Job kind / stage 表（已实现部分）

`queued` 是初始 stage；终态由 status 表达。stage code 是 API 契约，
可新增值，不得改变既有语义。

| Job kind | 状态 | 有序阶段（stage: progress） |
| --- | --- | --- |
| `lesson_import` | **B5 已实现** | `acquiring:10 → detecting:20 → extracting:40 → normalizing:55 → chunking:70 → validating:85 → publishing:95` |
| `lesson_regenerate` | B6 | `loading_context → chunking → generating_teacher_outline → generating_student_notes → validating → publishing` |
| `quiz_generate` / `review_quiz_generate` | B6 | 见 03 §6.4 |
| `attempt_grade` | B6 | 见 03 §6.4 |
| `wrong_answer_analyze` / `summary_evaluate` | B6 | 见 03 §6.4 |
| `claude_connectivity_test` | B6 | `connecting → validating` |
| `lesson_export` | B7 | `collecting → rendering → packaging → publishing` |

lesson_import 的 `generating_teacher_outline/generating_student_notes`
需要 Claude，B6 接入；B5 “按需跳过”，stage 序列中不出现。

## 3. lesson_import 管线行为

### 3.1 素材获取（acquiring）

| kind | 行为 |
| --- | --- |
| `text` | 内联文本写入任务目录，类型固定 `txt` |
| `upload` | 校验 file_object 为 ready 后流式复制进任务目录，强制 `MAX_DOWNLOAD_BYTES` 上限（超出 → `material_too_large`） |
| `url` | 见 3.4 URL 管线 |

### 3.2 类型检测（detecting）

以文件 Magic 为准、扩展名为辅，不信任扩展名（03 §8.3）：
PDF（`%PDF-`）、DOCX（zip 且含 `word/document.xml`，或 `.docx/.doc`
扩展名）、VTT（`WEBVTT` 头）、SRT（序号+时间轴模式）、HTML
（doctype/`<html>`/head+body 特征）、音视频（ftyp/EBML/ID3/RIFF-WAVE/
fLaC/OggS，audio/video 结合扩展名区分）、合法 UTF-8 文本兜底为
`txt`（`.md/.markdown` 为 `md`）。无法识别 → `material_unsupported_type`。

### 3.3 提取 / 规范化 / 分块

- 提取器版本（写入素材 metadata，算法变更必须升级版本号）：
  `txt/v1`、`srt/v1`、`vtt/v1`、`docx/v1`（标准库 zip+XML，无外部依赖）、
  `html/v1`（去 script/style/注释/不可见容器/标签 + 实体反转义）、
  `pdf/v1`（外部 `pdftotext -layout -enc UTF-8 <in> -`）、
  `subtitle/v1`（URL 字幕清洗）、`whisper/v1`、`media/v1`。
- 规范化 `norm/v1`：与 legacy `clean_text` 逐字节一致（golden fixture
  `testdata/legacy-fixtures/text-processing.golden.json` 冻结，期望值经
  legacy Python 运行时实际核对）。
- 分块 `chunk/v1`：按空行段落贪心装箱，目标 1200 rune，超长段落按
  rune 边界硬切；同一输入逐字节可复现。**chunk 不落库**：Job result
  只存 `chunk_count` 与 `chunks_sha256` 清单聚合 hash；B6 生成任务以
  同一版本从 active raw_text 文档重新分块并用该 hash 校验复现。
- 文本上限 `MaxTextRunes`（默认 2,000,000 rune，超出 →
  `material_too_large`）；清洗后为空 → `material_empty_content`。

### 3.4 URL 管线（字幕优先）

1. Worker 二次 SSRF 校验（API 创建时的校验不能覆盖 DNS 重绑定）：
   仅 https、host 白名单、DNS 解析结果全部公网；
2. 预检请求跟随重定向，**逐跳**重复上述校验，最多 5 跳，只读响应头
   与 ≤1KB 前缀；命中私网/非白名单/非 https → `url_blocked`；
3. `yt-dlp --version` 记录工具版本；
4. 人工字幕：`yt-dlp --skip-download --write-subs --no-write-auto-subs
   --sub-langs <偏好置前> --sub-format vtt/srt/lrc/best --no-playlist
   --socket-timeout 30 --retries 3 --max-filesize <N> --write-info-json
   -o <workdir>/src.%(ext)s <url>`；
5. 无人工字幕再试自动字幕（`--write-auto-subs --no-write-subs`）；
6. 仍无字幕：`transcription_mode=whisper_local`（payload 覆盖进程配置
   `TRANSCRIPTION_MODE`）时下载音轨（`yt-dlp -x --audio-format wav`）
   并 `whisper-cli -m <WHISPER_MODEL_PATH> -f audio.wav -otxt
   -of transcript` 本地转写；否则终态 `transcript_required`（可行动
   detail：上传字幕、粘贴文本或配置 whisper_local）；
7. `info.json` 只取 title/duration/webpage_url/extractor 写入素材
   metadata（不保存完整 JSON）。

上传音视频：先 `ffmpeg -i <in> -map 0:s:0 -f srt embedded.srt -y` 抽
内嵌字幕；失败则按转写模式走 whisper（`ffmpeg -i <in> -vn -acodec
pcm_s16le -ar 16000 -ac 1 audio.wav -y`，与 legacy 参数一致）或
`transcript_required`。同目录字幕不适用（任务目录按 job_id 隔离）。

### 3.5 发布（publishing，Complete 同事务）

- `documents` 新增 `raw_text` 版本并置 active（旧版本保留，
  `uq_documents_active` 保证每 lesson+kind 单 active）；
- 素材 `status=ready`，metadata 合并 `detected_type/parser_version/
  normalize_version/chunk_version/char_count/chunk_count[/source]`；
- 课时 `status=ready`；
- Job result：`{material_id, document_id, document_revision,
  detected_type, parser_version, normalize_version, chunk_version,
  char_count, chunk_count, chunks_sha256}`——不含材料正文。

**失败/取消不改变任何 active 文档**（发布是单事务，崩溃即整体回滚）。

### 3.6 失败/取消的业务侧写

| 终态 | 素材 | 课时 |
| --- | --- | --- |
| `failed` | `status=failed`，metadata 记录 `error_code` | `failed` |
| `cancelled` | 保持 `pending`（未处理，不污名化） | 有发布文档回 `ready`，否则回 `draft` |
| requeue（自动重试） | 不变 | 不变 |

## 4. 素材错误码（jobs.error_code）

| 错误码 | 可重试 | 含义与建议 |
| --- | --- | --- |
| `url_blocked` | 否 | Worker 二次 SSRF 校验失败（私网/非白名单/非 https/重定向链违规） |
| `material_fetch_failed` | 按原因 | 下载或外部工具失败；429/5xx/超时/连接错误自动重试；412 反爬、需登录、URL 不支持为终态并给出替代动作 |
| `transcript_required` | 否 | 无字幕且未配置转写；detail 含修复动作（上传字幕/粘贴文本/配置 whisper_local） |
| `tool_missing` | 否 | 外部二进制缺失（yt-dlp/ffmpeg/whisper-cli/pdftotext）或 WHISPER_MODEL_PATH 未配置 |
| `material_unsupported_type` | 否 | 无法识别的文件类型 |
| `material_parse_failed` | 否 | 文件损坏/非 UTF-8/DOCX 结构损坏/PDF 解析失败/素材已删除 |
| `material_empty_content` | 否 | 清洗后无有效文本 |
| `material_too_large` | 否 | 下载或规范化文本超过上限 |
| `job_timeout` | 是 | 超过任务总时限（`WORKER_JOB_TIMEOUT`） |
| `publish_failed` | 是 | 发布事务失败（如数据库竞争） |
| `internal_error` | 是 | 未分类内部错误 |
| `unsupported_job_kind` | 否 | 该 kind 尚未注册 Handler（B6/B7 补） |
| `worker_lost` | 自动 | 租约过期回收（Recovery 写入；次数耗尽进 failed） |

## 5. 外部进程契约（proc.Runner）

- 仅以参数数组调用（`exec.CommandContext`），无 shell；取消/超时即
  Kill；stdout ≤32MB、stderr ≤1MB 有界采集，错误摘要只带 stderr
  尾部 300 字节（不含完整输出，避免泄漏材料片段）。
- 二进制名（Handler 字段，可用环境相应配置覆盖）：`yt-dlp`、
  `ffmpeg`、`whisper-cli`、`pdftotext`。Worker 镜像需求见
  `05-deployment-operations.md` §7.2。
- 测试注入 `proc.Fake`（FIFO 编排 + 调用记录），不要求真实二进制
  与外网。

## 6. Worker 配置项

| 环境变量 | 默认 | 说明 |
| --- | --- | --- |
| `WORKER_CONCURRENCY` | 1 | 并行 Job 数 |
| `JOB_LEASE_DURATION` | 60s | 租约时长（心跳为其 1/3） |
| `WORKER_JOB_TIMEOUT` | 30m | 单任务总时限 |
| `MAX_DOWNLOAD_BYTES` | 2GiB | 下载/上传复制上限 |
| `URL_ALLOWLIST` | 空 | URL 素材主机白名单（空 = 全拒） |
| `TRANSCRIPTION_MODE` | `subtitle_only` | `subtitle_only` / `whisper_local` |
| `WHISPER_MODEL_PATH` | 空 | whisper_local 模型路径 |

## 7. 演示数据（seed）

```bash
make seed   # 幂等；数据库配置与 api/worker 相同
```

- 课程 `demo-course`；课时 1「算法复杂度入门」ready（含已发布
  raw_text 文档与 ready 文本素材）；
- 课时 2「二分查找」processing + 一个 queued `lesson_import` Job
  （jobs 幂等键 `seed/demo-lesson2-import-v1`）——启动 Worker 后可
  观察完整阶段事件流与文档发布。

## 8. 已知限制（诚实清单）

- URL 预检与 yt-dlp 下载之间存在 DNS 重绑定 TOCTOU 窗口：白名单
  域名 + 下载前即时解析把残余风险限制在“白名单站点自身被重定向到
  私网”场景；yt-dlp 进程侧不做网络隔离（B8 可用容器网络策略补强）。
- HTML 不可见容器移除是近似实现（RE2 不支持反向引用，对
  div/span/section 逐个匹配）；嵌套同标签的极端文档可能残留边界。
- `_clean_lrc` 不做整体空白规范化（与 legacy 一致）；统一由
  normalizing 阶段兜底。
- `lesson_export` 等未注册 kind 当前会被终态 `unsupported_job_kind`；
  若需保留排队，请在 B7 注册后再创建此类任务。
- Worker 被 SIGKILL 时 `tmp/{job_id}/` 工作目录无数据库行可追踪，
  周期 Recovery 不扫描 `tmp/`；残留目录需运维清理（B8 doctor 可加
  巡检）。Job 持有的 temporary file_objects 行不受影响，仍由文件
  Recovery 正常处置。
