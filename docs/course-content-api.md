# 课程内容管理 API

基础路径：`/api/v1`。完整字段定义、响应和错误格式见 [OpenAPI](../api/openapi.yaml)。本次新增 20 个接口，补齐课程子资源的管理能力；课时、素材导入、测验生成和作答保存等原有接口继续使用。

| 资源 | 新增 | 读取 | 修改 | 删除 |
| --- | --- | --- | --- | --- |
| 课时 | `POST /courses/{course_id}/lessons` | `GET /lessons/{lesson_id}` | `PATCH /lessons/{lesson_id}` | `DELETE /lessons/{lesson_id}` |
| 素材 | `POST /lessons/{lesson_id}/materials/text`、`/url`、`/upload` | `GET /materials/{material_id}`，课时素材列表沿用原接口 | `PATCH /materials/{material_id}` | `DELETE /materials/{material_id}` |
| 原文、学生笔记、教师大纲 | `POST /lessons/{lesson_id}/documents/{kind}` | `GET /lessons/{lesson_id}/documents/{kind}`，历史版本列表和版本详情沿用原接口 | `PUT /lessons/{lesson_id}/documents/{kind}` | `DELETE /documents/{document_id}` |
| 知识点 | `POST /lessons/{lesson_id}/knowledge-points` | `GET /knowledge-points/{knowledge_point_id}`，课程目录沿用原接口 | `PATCH /knowledge-points/{knowledge_point_id}` | `DELETE /knowledge-points/{knowledge_point_id}` |
| 测验及题目 | `POST /lessons/{lesson_id}/quizzes` | `GET /quizzes/{quiz_id}/editor`，学生详情和课时列表沿用原接口 | `PATCH /quizzes/{quiz_id}` | `DELETE /quizzes/{quiz_id}` |
| 学习总结评估 | `POST /lessons/{lesson_id}/summary-evaluations` | `GET /lessons/{lesson_id}/summary-evaluations`、`GET /summary-evaluations/{evaluation_id}` | `PATCH /summary-evaluations/{evaluation_id}` | `DELETE /summary-evaluations/{evaluation_id}` |
| 作答记录 | `POST /quizzes/{quiz_id}/attempts` | `GET /attempts/{attempt_id}`，课时历史列表沿用原接口 | `PATCH /attempts/{attempt_id}/answers/{question_id}` | `DELETE /attempts/{attempt_id}` |
| 薄弱点 | `POST /courses/{course_id}/weaknesses` | `GET /weaknesses`、`GET /weaknesses/{weakness_id}` | `PATCH /weaknesses/{weakness_id}` | `DELETE /weaknesses/{weakness_id}` |

`kind` 为 `raw_text`、`student_notes` 或 `teacher_outline`。进度、掌握度和判分由数据计算，仍通过原学习/评估流程更新。

## 请求规则

- 新增接口要求 UUID 格式的 `Idempotency-Key`；同一用户意图重试应复用相同键。修改素材和修改学习总结会创建后台任务，也要求该请求头。
- 素材、知识点、测验、薄弱点的 PATCH 请求必须带读取结果中的 `version`。过期版本返回 `409 version_conflict`，客户端应重新读取后合并修改。
- 文档 PUT 必须带 `expected_revision`，成功后生成新文档 ID 和 revision；未删除的历史版本保持可读，删除后修订号不会复用。首次创建用 POST，已有当前版本时 POST 返回 409。
- 同一课时正在导入、生成、评估或批阅时，相关手工写入返回 `409 resource_busy`，避免后台结果覆盖手工修改。维护模式下拒绝写入。
- 知识点需要解释、学习目标和当前原文中的连续引用。修改或重新导入原文后旧知识点停用，下一次出题会重新分析，也可以手工添加。

## 测验编辑

`GET /quizzes/{quiz_id}/editor` 返回含标准答案与解析的管理表示；原学生详情和作答接口的答案可见性规则不变。

```json
{
  "title": "二分查找练习",
  "generation_notes": "重点考边界条件",
  "questions": [
    {
      "type": "single_choice",
      "stem": "二分查找适用于哪种输入？",
      "options": {"A": "按一致规则排序的数组", "B": "任意数组"},
      "correct_answer": "A",
      "explanation": "需要有序输入才能按比较结果排除一半区间。",
      "tested_concepts": ["二分查找的适用前提"],
      "difficulty": "easy",
      "cognitive_level": "apply",
      "score": 5
    }
  ]
}
```

上述内容用于 `POST /lessons/{lesson_id}/quizzes`。编辑时 PATCH 带 `version` 以及要修改的字段。只修改标题或备注时 ID 不变；提供完整 `questions` 数组时，新增、删除、修改和排序题目一起完成，响应返回新的测验 ID，旧测验归档并从列表移除，历史作答继续使用旧题目。客户端须采用响应中的新 ID 和 version。

## 总结和删除行为

总结 PATCH 请求为 `{"summary":"修订后的学习总结"}`，保留旧总结和评估，返回新的 `evaluation.id` 与评估任务（202）；GET 的 `summary_text` 可读取原总结，课时总结历史支持游标分页。

删除文档时只删除指定版本；删除当前版本后不自动回退旧版本。删除知识点不改写历史试卷快照。删除课时会一并删除素材、文档、知识点、测验、作答、总结和课时薄弱点，并重新计算课程进度；单项和批量删除使用相同的级联规则。删除测验也会直接删除其作答、答案、来源进度快照及关联证据，并重新计算学习进度，无需逐项手动删除。存在进行中的生成、总结评估或批阅任务时仍暂时拒绝删除，等待任务完成或取消后重试。删除作答会移除答案、该作答产生的掌握度快照和关联证据，并根据剩余数据追加当前进度快照；人工维护的薄弱点状态保留。删除薄弱点也会重新计算相关进度。后台任务审计保留。

## 页面删除入口

课程列表与课时列表每行右侧直接显示“删除”，也保留“选择 → 删除所选”的批量入口。课时内的测验、知识点、作答记录和总结历史均支持单项删除；笔记、原文、大纲在阅读区标题旁删除当前查看的版本，历史版本仍可从“历史版本”打开。原文页下方的“来源素材”可删除导入源，薄弱点列表也有独立删除按钮。

所有入口先显示确认框，确认成功后刷新对应列表和进度；删除失败保留确认框并展示处理提示。总结历史来自服务端，删除已选总结会清除本地选择并保留未提交的输入草稿。
