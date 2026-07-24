# legacy-fixtures：旧 Python 行为基线与迁移输入夹具

本目录是 Web 重构 B0 阶段冻结的旧系统行为基线。**全部是人工编写的脱敏示例数据**，
不包含任何真实学习材料、个人信息、API Key 或宿主机绝对路径。

## 内容

| 路径 | 用途 | 消费者 |
| --- | --- | --- |
| `question-policy.golden.json` | 题量、认知层次/难度分布、加权掌握度、复杂度启发式的 golden 用例 | `legacy/python/tests/test_golden_fixtures.py`；B4 阶段 Go `internal/quiz/policy` 表驱动测试 |
| `objective-grading.golden.json` | 客观题答案规范化与判分 golden 用例 | 同上（Go `attempt` 判分） |
| `weakness.golden.json` | 薄弱点 ID、合并、降严重度、标记掌握 golden 用例 | 同上（Go `mastery` 模块） |
| `mastery.golden.json` | 章节掌握度公式与连续对错统计 golden 用例 | 同上 |
| `course-minimal/` | 最小脱敏课程数据，目录布局镜像 `legacy-data/`（`course/` + `.tasks/`） | B9 `legacy-import` 的 scan/import 测试输入 |

## 规则

1. **禁止从 `legacy-data/` 复制内容**。`legacy-data/` 是被 git 忽略的真实用户数据，只读；
   本目录必须保持可提交、可公开。新增用例一律人工编写，主题限定在公开示例（如“极限”）。
2. golden JSON 由 `legacy/python/tools/generate_golden_fixtures.py` 从当前实现生成，
   生成后即为冻结基线。**只有在行为变更是有意且经过评审时**才允许重新生成；
   重新生成必须在提交说明中解释行为差异。
3. 时间字段（`first_seen`/`last_seen` 等墙钟时间）不参与 golden 比较。
4. 浮点数以 JSON 完整双精度保存；跨语言（Go）比较建议使用 1e-12 绝对容差。
5. golden 如实冻结 legacy 现状，包括已知怪癖（用例名带 `legacy_quirk`，文件内 `notes` 有说明）。
   目标系统的不同行为（如 `03-backend-architecture.md` §10.2 的多选集合判分、§10.1 的 v2 策略）
   由 B4 显式决策，不得悄悄“修正” golden。
6. `course-minimal/` 的 `output_dir` 等路径字段使用相对路径，仅为格式示例；
   真实旧数据中这些字段是宿主机绝对路径，Importer 不得将其用于业务引用。

## 校验

```bash
make test-legacy      # 旧 Python 测试 + golden 用例 + fixture schema 校验
```
