# ops-mcp — 常用构建 / 测试 / 镜像 / 打包目标
# 门禁对齐 cursor.md §6.1 / §8

APP          := ops-mcp
CMD          := ./cmd/server
BIN_DIR      := bin
BINARY       := $(BIN_DIR)/$(APP)
CONFIG       ?= ./config/ops-mcp.yaml
GO           ?= go
DOCKER_IMAGE ?= ops-mcp:latest
DOCKER_PORT  ?= 20267

VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOOS         ?= $(shell $(GO) env GOOS)
GOARCH       ?= $(shell $(GO) env GOARCH)
DIST_DIR     := dist
PACKAGE_NAME := $(APP)-$(VERSION)-$(GOOS)-$(GOARCH)
STAGING      := $(DIST_DIR)/$(PACKAGE_NAME)
TARBALL      := $(DIST_DIR)/$(PACKAGE_NAME).tar.gz

.DEFAULT_GOAL := help

SNMP_COMPOSE := deploy/dev-snmp/docker-compose.yml
NET_COMPOSE  := deploy/dev-net/docker-compose.yml

.PHONY: help all build test vet race check run run-dev integration tidy clean docker docker-run tar \
	snmp-up snmp-down snmp-restart snmp-ps snmp-logs \
	net-up net-down net-restart net-ps net-logs

help: ## 显示可用目标
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  %-14s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

all: check ## 完整门禁（vet + test + build）

build: ## 编译二进制到 bin/ops-mcp
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -trimpath -ldflags="-s -w" -o $(BINARY) $(CMD)

test: ## 运行单元测试 go test ./...
	$(GO) test ./...

vet: ## 静态检查 go vet ./...
	$(GO) vet ./...

race: ## 竞态检测 go test -race ./...
	$(GO) test -race ./...

check: vet test build ## 最低测试门禁（vet → test → build）

run: build ## 编译并启动服务
	$(BINARY) --config $(CONFIG)

run-dev: ## 直接 go run 启动（开发用）
	$(GO) run $(CMD) --config $(CONFIG)

integration: ## 集成测试（需 OPS_MCP_INTEGRATION=1 与本地凭据）
	OPS_MCP_INTEGRATION=1 $(GO) test ./internal/connector/... -count=1

tidy: ## go mod tidy
	$(GO) mod tidy

clean: ## 删除构建产物与发行包
	rm -rf $(BIN_DIR) $(DIST_DIR)

docker: ## 构建 Docker 镜像
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## 运行容器（挂载本地 config/plugins，端口见 DOCKER_PORT）
	docker run --rm -p $(DOCKER_PORT):$(DOCKER_PORT) \
		-v "$(CURDIR)/config:/app/config:ro" \
		-v "$(CURDIR)/plugins:/app/plugins:ro" \
		-e OPS_MCP_AUTH_TOKEN \
		$(DOCKER_IMAGE)

snmp-up: ## 启动本地 SNMP 模拟器（UDP 1161，见 deploy/dev-snmp）
	docker compose -f $(SNMP_COMPOSE) up -d

snmp-down: ## 停止并删除本地 SNMP 模拟器
	docker compose -f $(SNMP_COMPOSE) down

snmp-restart: ## 重启本地 SNMP 模拟器（改 .snmprec 后用）
	docker compose -f $(SNMP_COMPOSE) up -d --force-recreate

snmp-ps: ## 查看本地 SNMP 模拟器状态
	docker compose -f $(SNMP_COMPOSE) ps

snmp-logs: ## 跟踪本地 SNMP 模拟器日志
	docker compose -f $(SNMP_COMPOSE) logs -f --tail=100

net-up: ## 启动本地 TCP/UDP echo（19090/19091，见 deploy/dev-net）
	docker compose -f $(NET_COMPOSE) up -d

net-down: ## 停止并删除本地 TCP/UDP echo
	docker compose -f $(NET_COMPOSE) down

net-restart: ## 重启本地 TCP/UDP echo
	docker compose -f $(NET_COMPOSE) up -d --force-recreate

net-ps: ## 查看本地 TCP/UDP echo 状态
	docker compose -f $(NET_COMPOSE) ps

net-logs: ## 跟踪本地 TCP/UDP echo 日志
	docker compose -f $(NET_COMPOSE) logs -f --tail=100

tar: ## 构建并打包发行包到 dist/*.tar.gz（可 GOOS/GOARCH 交叉编译，不覆盖本机 bin/）
	@rm -rf "$(STAGING)"
	@mkdir -p "$(STAGING)/bin" "$(STAGING)/config" "$(STAGING)/deploy" "$(STAGING)/plugins"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -trimpath -ldflags="-s -w" -o "$(STAGING)/bin/$(APP)" $(CMD)
	@cp -a plugins/. "$(STAGING)/plugins/"
	@cp config/*.example "$(STAGING)/config/"
	@mkdir -p "$(STAGING)/config/openapi"
	@cp -a config/openapi/. "$(STAGING)/config/openapi/"
	@cp deploy/ops-mcp.service deploy/install.sh deploy/uninstall.sh deploy/README.md "$(STAGING)/deploy/"
	@cp README.md "$(STAGING)/"
	@chmod 0755 "$(STAGING)/bin/$(APP)" "$(STAGING)/deploy/install.sh" "$(STAGING)/deploy/uninstall.sh"
	@tar -czf "$(TARBALL)" -C "$(DIST_DIR)" "$(PACKAGE_NAME)"
	@echo "packed: $(TARBALL)"
	@ls -lh "$(TARBALL)"
