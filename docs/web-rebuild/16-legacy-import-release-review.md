# 旧数据导入与发布审查

日期：2026-07-24  
发布契约版本：OpenAPI `0.3.0-draft`  
Importer：`legacy-import-v1`

## 结论

**Web/API/Worker 发布：维持既有 GO。旧数据最终切换：NO-GO。**

Importer 本身可发布到手工 profile：扫描只读，导入在一个 PostgreSQL 事务内完成，并且每一源文件都有审计记录。真实归档演练仍有 7 条隔离记录；在逐条决定隔离或清理并重新生成 Manifest 前，不能把旧 Python 写入停掉或宣称历史数据已完整迁移。

## 导入报告

真实 `legacy-data/` 只读预切换演练的 Manifest：5 门课程、9 个课时、107 个文件、10 份 Quiz、2 份 Attempt、2 份错题证据、1 份弱项、1 份聚合成绩、38 个任务。Manifest SHA-256 为 `d1bde4c7d689d7766c4042859844701452155ea1bb17610ab482ba96e2fc0e80`。

导入批次结果：107/107 记录完成对账，85 `imported`、15 `skipped_by_policy`、7 `quarantined`，状态 `needs_review`。隔离原因：

- 4 份 Quiz 的主观题 `correct_answer=null`，违反当前题目领域约束，不能伪造答案。
- 同一作答有 `feedback` 和 `grade_result` 两个候选，未定义唯一性，因此两份均为 `ambiguous_feedback`。
- 1 条 `scores.json` 聚合成绩无法唯一匹配 Attempt，记录为 `unmatched_score`。

10 个 pending/failed 历史任务被审计为 `legacy_task_not_resumable`；不会创建 Job。导入报告、数据库审计和日志均不保留 Prompt、学习正文、答案或 API Key。

## 稳定接口

- CLI：`legacy-import scan|import --source <dir> --report <file> [--resolution <file>] [--dry-run] [--fail-on-warning]`。
- `scan` 不连接数据库、不写源目录或数据卷；报告通过临时文件和原子 rename 发布。
- `Resolution` schema v1：`manifest_sha256` 与每条 `{code, relative_path}` 必须精确匹配 Manifest error finding；其 SHA-256 存入 `legacy_import_batches.resolution_sha256`。
- 批次/记录表是内部审计接口，不加入 OpenAPI，也不经 REST/SSE 发给前端。前端无需处理导入过程或学习正文泄漏风险。
- `grading_failed` + `legacy_task_not_resumable` 是前端可展示的既有 Attempt 错误语义；用户可经现有 API 明确重新发起当前批阅，Importer 不续跑历史任务。

## 发布审查

| 范围 | 结果 | 审查结论 |
| --- | --- | --- |
| PRD 与迁移路线 | 通过 | 单用户、本机、只读旧目录、隔离而不伪造与 `07-migration-roadmap.md` 一致。 |
| OpenAPI/前端 | 通过 | Importer 是手工运维边界，未扩展浏览器契约；历史 Attempt 继续使用既有 `grading_failed` 错误码。 |
| 并发与事务 | 通过 | Manifest advisory lock、`(manifest,resolution)` 唯一索引、业务写入/审计/最终对账同一事务；相同 source hash 去重。 |
| AI 故障 | 通过 | Importer 不调用 Claude、不创建 Job；历史 pending/failed 只迁移可验证答案和失败语义。默认测试使用 fake Provider。 |
| 安全 | 通过 | `/legacy:ro`、非 root、只读 rootfs、无 Anthropic secret；扫描和报告不含正文、答案、Prompt、密码或 Key。 |
| Compose/恢复 | 通过 | 手工 profile 仅挂载 PostgreSQL secret；既有 fake 闭环、Worker 强杀、doctor、备份/空卷恢复和 SSE 代理演练通过。 |
| 前端交接 | 通过 | 前端只需继续按 OpenAPI/Job SSE 使用现有错误语义，不需要导入管理页或导入数据访问权。 |

## 预切换和回滚演练

预切换：使用隔离 PostgreSQL 运行真实归档导入，结果如上；源目录的文件、mtime、权限均未修改。最终切换前必须停止 Python 写入，重新运行 scan，审核新报告，再以 `--fail-on-warning` 执行 import。

回滚：在尚未切换旧入口前停止新服务、继续使用保留的 `legacy-data/` 与 Python 入口；新库按既有 `make docker-backup` / `RESTORE_BACKUP=... make docker-restore` 恢复。Importer 从不回写旧目录，因此回滚不需要反向文件迁移。

## 验证记录

```text
go test ./internal/legacyimport -count=1 -v
PASS: fixture idempotency, incomplete-attempt quarantine, feedback validation,
      empty subject marker, resolution replay, real legacy-data pre-cutover rehearsal

go test ./cmd/legacy-import -count=1
PASS

go run ./cmd/legacy-import scan --source legacy-data --report /private/tmp/ai-study-tutor-legacy-scan.json --fail-on-warning
scan completed; 107 files; 0 scan findings
```

## 残余风险和下一步

- 4 份缺少主观题参考答案的历史 Quiz 必须由用户选择保留隔离、补充可验证答案，或放弃迁移；不能由 Importer 自动补全。
- 两个批阅候选和 1 条聚合成绩需人工对账后决定是否保留为隔离审计。resolution 只接受 scanner finding，不会把 runtime quarantine 强行标为成功。
- 最终切换前需重新运行 scan，因为任何源 hash 变化都会生成新的 Manifest；未重新审核的旧 resolution 不可复用。
- 本机存在早先测试遗留的嵌入式 PostgreSQL 进程；完整 `make verify` 的 race 阶段在 `internal/quiz` 初始化临时 PostgreSQL 时阻塞。`go vet`、构建、文档/OpenAPI 门禁和全部 legacy-import race/集成测试均已通过。CI/发布机应先清理孤立测试进程或使用受控 PostgreSQL，再将全量 race 结果作为最终发布门禁。
