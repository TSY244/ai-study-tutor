# AI Study Tutor Web 重构工程命令
#
# 契约与文档门禁：
#   make generate-openapi  从设计源展开公共响应头，生成发布契约 api/openapi.yaml
#   make verify-openapi    校验发布契约（X-Request-ID 逐响应、引用、operationId、无漂移）
#   make check-openapi-breaking  对比基线，阻止操作/请求/响应/Schema 破坏性变更
#   make verify-docs       校验 web-rebuild 文档集与设计源声明完整
#   make seed            写入前端联调演示数据（幂等）
# 旧系统基线：
#   make test-legacy       运行 legacy/python 全部测试（含 golden fixture 用例）
# Go 质量门禁（B1 起）：
#   make go-build go-fmt go-vet go-test
# 汇总：
#   make verify            全部文档/契约/Go 质量门禁

GO_PKGS := ./cmd/... ./internal/...

.PHONY: generate-openapi verify-openapi check-openapi-breaking verify-docs test-legacy \
	go-build go-fmt go-vet go-test verify seed docker-init docker-config \
	docker-build docker-up docker-seed docker-doctor docker-backup docker-restore \
	docker-maintenance-repair docker-legacy-scan docker-verify-secrets docker-smoke \
	docker-legacy-import docker-drill docker-down

generate-openapi:
	uv run --script tools/openapi/openapi_contract.py generate

verify-openapi:
	uv run --script tools/openapi/openapi_contract.py verify

check-openapi-breaking:
	uv run --script tools/openapi/openapi_contract_test.py
	uv run --script tools/openapi/openapi_contract.py breaking

verify-docs:
	uv run --script tools/docs/verify_docs.py

# TUTOR_CONFIG 指向已提交的示例配置，避免依赖被 gitignore 的本地 config.json
test-legacy:
	cd legacy/python && TUTOR_CONFIG=config.example.json uv run --extra dev pytest -q

go-build:
	go build $(GO_PKGS)

go-fmt:
	@test -z "$$(gofmt -l cmd internal)" || { echo "以下文件需要 gofmt："; gofmt -l cmd internal; exit 1; }

go-vet:
	go vet $(GO_PKGS)

go-test:
	go test -race $(GO_PKGS)

# 写入前端联调演示数据（幂等；数据库配置与 api/worker 相同）
seed:
	go run ./cmd/seed

verify: verify-docs verify-openapi check-openapi-breaking go-fmt go-vet go-test go-build

docker-init:
	./deploy/scripts/init.sh

docker-config: docker-init
	docker compose --env-file deploy/.env -f deploy/compose.yaml config --quiet

docker-build: docker-config
	docker compose --env-file deploy/.env -f deploy/compose.yaml build

docker-up: docker-config
	docker compose --env-file deploy/.env -f deploy/compose.yaml up -d --build --wait

docker-seed:
	docker compose --env-file deploy/.env -f deploy/compose.yaml --profile tools run --rm seed

docker-doctor:
	docker compose --env-file deploy/.env -f deploy/compose.yaml --profile tools run --rm doctor

docker-backup:
	docker compose --env-file deploy/.env -f deploy/compose.yaml --profile tools run --rm backup

docker-restore:
	@test -n "$(RESTORE_BACKUP)" || { echo "需要 RESTORE_BACKUP=backup-..."; exit 2; }
	RESTORE_BACKUP=$(RESTORE_BACKUP) RESTORE_CONFIRM=RESTORE docker compose --env-file deploy/.env -f deploy/compose.yaml --profile manual run --rm restore

docker-maintenance-repair:
	docker compose --env-file deploy/.env -f deploy/compose.yaml --profile manual run --rm maintenance-repair

docker-legacy-scan:
	docker compose --env-file deploy/.env -f deploy/compose.yaml --profile manual run --rm legacy-import

# 调用者必须显式提供 resolution；默认命令保持只读 scan。
docker-legacy-import:
	@test -n "$(LEGACY_IMPORT_ARGS)" || { echo "需要 LEGACY_IMPORT_ARGS='import --source /legacy --report /reports/import.json --resolution /reports/resolution.json'"; exit 2; }
	docker compose --env-file deploy/.env -f deploy/compose.yaml --profile manual run --rm legacy-import $(LEGACY_IMPORT_ARGS)

docker-verify-secrets:
	./deploy/scripts/verify-secrets.sh

docker-smoke:
	./deploy/scripts/release-smoke.sh

docker-drill:
	RUN_DEPLOYMENT_DRILLS=1 ./deploy/scripts/drill.sh

docker-down:
	docker compose --env-file deploy/.env -f deploy/compose.yaml down
