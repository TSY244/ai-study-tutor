# 发布 OpenAPI 契约

`api/openapi.yaml` 是服务端和前端代码生成的**唯一发布契约**（OpenAPI 3.1）。

## 生成

```bash
make generate-openapi
```

- 输入（设计源）：`docs/web-rebuild/openapi.yaml`
- 展开器：`tools/openapi/openapi_contract.py generate`（经 `uv run --script` 运行，依赖固定为 `pyyaml==6.0.2`）
- 输出：本目录的 `openapi.yaml`

生成器把设计源顶层 `x-common-response-headers`（目前为 `X-Request-ID`）展开进
**每一个成功和错误 Response**（包括内联 Response 与 `components.responses`），
随后移除 `x-common-response-headers` / `x-generation-rules` 两个设计源扩展。
Response 中已显式声明的同名 Header 优先，不会被公共值覆盖。

输出是确定性的：固定依赖版本与序列化参数，同一设计源必然得到逐字节相同的
发布文件；首次运行需要网络以下载被固定的 PyYAML，之后使用 uv 缓存。

## 禁止手改

1. **禁止手工编辑 `api/openapi.yaml`**。文件头部有 GENERATED 标记；
   `make verify-openapi` 会把发布文件与“从设计源现场展开的结果”做逐字节比较，
   任何手改或漂移都会失败。
2. 修改契约只能改设计源 `docs/web-rebuild/openapi.yaml`，然后重新运行
   `make generate-openapi` 并一起提交两个文件。
3. 前端类型/Client 只从本目录的发布契约生成，**不得直接读取设计源**；
   生成客户端必须保留原始响应 Header（`X-Request-ID` 在所有响应上）。
4. 契约演进只做向后兼容的字段新增；删除/重命名/改语义按
   `docs/web-rebuild/04-api-contract.md` §17 处理。

## 校验

```bash
make verify-openapi   # 逐响应检查 X-Request-ID、引用可解析、operationId 唯一、产物无漂移
make check-openapi-breaking
make verify-docs      # 设计文档与发布规则完整性
```

`make check-openapi-breaking` 默认对比已发布快照 `api/openapi.baseline.yaml`；可用
`OPENAPI_BASELINE=/path/to/base.yaml make check-openapi-breaking` 显式覆盖。快照不存在时
回退到 `HEAD:api/openapi.yaml`。
它会阻止路径/方法/响应、operationId、必需参数、请求体、响应媒体/Schema、
Schema 属性/必需字段/枚举的破坏性变更，并先运行聚焦自测试。没有任何可用
基线时会显式 `SKIP`，避免把“无基线”误报为兼容。

## 前端联调包

```bash
make generate-openapi          # 生成 api/openapi.yaml
cd apps/web && npm run api:gen # 生成 TypeScript Client 与 MSW handlers
make seed                      # 写入幂等演示数据，默认 API/Worker DB 配置
```

SSE 客户端示例位于
`apps/web/src/api/job-stream.ts`，使用 `fetch` 传输、`Last-Event-ID` 游标、
`stream.resync_required` REST 对账和无 `id` 心跳。手工帧示例见
`api/examples/sse/`。生成脚本会按 `text/event-stream` 媒体类型从 Orval REST
client/MSW 中排除 SSE 操作，避免把常驻流误生成为一次性 `Promise`；Envelope
类型仍由 OpenAPI 生成。
