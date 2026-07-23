# ops-mcp — Agent 开发上下文

> 本文件供 Cursor Agent / 人类贡献者在开发前阅读。  
> **任何开发、修改、重构任务，必须以测试通过作为完成条件；未通过测试不得宣称任务完成。**

---

## 1. 项目是什么

**ops-mcp**：自研 Remote MCP Server（Go）。AI Agent（Claude Code / Codex / ChatGPT Agent 等）通过 HTTP JSON-RPC（`POST /mcp`）调用运维能力。

**核心理念：Plugin 即能力。** 未加载 Plugin = 无对应 MCP Tool。不以 RBAC / allow_rules 做能力边界（MVP 不做权限与审计）。

| 层 | 职责 |
|----|------|
| Go | MCP 协议、Plugin 加载、YAML/OpenAPI 解析、参数校验、Goja、SSH/DB/Docker/Redis/Kafka/HTTP/Command/SNMP/TCP/UDP Connector |
| JS (`main.js`) | 磁盘 Plugin：参数编排、调用 `ctx.*` Connector、返回结果 |
| YAML (`plugin.yml`) | 磁盘 Plugin：Tool 元数据、input schema、timeout、target |
| `apis.yaml` + OpenAPI | 自动生成 MCP Tool（不经 Goja）；契约见 [docs/apis.md](docs/apis.md) |
| `commands.yaml` | 本机可执行文件白名单（Command Connector）；契约见 [docs/connector.md](docs/connector.md) |

详细设计见 `docs/`，不要凭记忆发明与文档冲突的契约。

---

## 2. 必读文档（按优先级）

| 顺序 | 文档 | 何时读 |
|------|------|--------|
| 1 | 本文件 `cursor.md` | 每次开发会话开始 |
| 2 | [docs/roadmap.md](docs/roadmap.md) | 确认当前 Phase 与验收项 |
| 3 | [docs/architecture.md](docs/architecture.md) | 改架构 / 跨层改动 |
| 4 | [docs/api.md](docs/api.md) | 改 HTTP / MCP 接口 |
| 5 | [docs/plugin.md](docs/plugin.md) + [docs/runtime.md](docs/runtime.md) | 写/改 Plugin 或 Goja |
| 6 | [docs/connector.md](docs/connector.md) + [docs/configuration.md](docs/configuration.md) | 写/改 Connector 或配置 |
| 7 | [docs/apis.md](docs/apis.md) | 写/改 HTTP API / OpenAPI → MCP Tool |

根 [README.md](README.md) 为对外总览；**实现细节以 `docs/*` 为准**。

---

## 3. 技术栈与目录

- **Go ≥ 1.24**，Gin，Goja，Viper，Zap，Cobra
- 模块路径：`github.com/fsj00/ops-mcp`
- 包布局（已实现）：

```
cmd/server/          # 入口
internal/
  api/               # Gin 路由（/mcp、/api/*、/health、嵌入 Web UI）
  mcp/               # JSON-RPC MCP
  plugin/            # Loader / Manager
  runtime/           # Goja
  executor/          # 校验 + 执行
  connector/{ssh,mysql,postgres,docker,redis,kafka,http,command,snmp,tcp,udp,netutil,dbutil,sqlguard}/
  openapi/           # OpenAPI 解析 / Discovery / Tool 生成（Phase 6）
  config/            # ConfigManager
  model/
web/                 # 嵌入式浏览器控制台（index.html + go:embed）
plugins/             # plugin.yml + main.js（linux/docker/mysql/postgres/redis/kafka/hosts/databases/apis/net/commands/snmp）
config/             # ops-mcp.yaml / hosts.yaml / databases.yaml / redis.yaml / kafka.yaml / apis.yaml / commands.yaml / snmp.yaml
deploy/              # systemd 安装脚本、dev-snmp、dev-net
docs/
```

新增能力 = 新增磁盘 Plugin 或 `apis.yaml` OpenAPI Tool（必要时加 Connector），不要在 MCP 层写死业务命令。

---

## 4. 硬性约束

### 4.1 安全与密钥

- **禁止**把真实密码、私钥、API Token 写入可提交文件、日志、测试快照、PR 描述、本文件。
- 真实凭据仅存在于本地：`config/hosts.yaml`、`config/databases.yaml`、`config/redis.yaml`、`config/kafka.yaml`、`config/apis.yaml`、`config/snmp.yaml`（已在 `.gitignore`）或环境变量。
- 仓库只保留 `*.yaml.example`（占位符 `CHANGE_ME`）。`commands.yaml` 通常无密钥，可本地维护；勿写入密钥材料。
- Connector / 日志必须脱敏 `password` / `private_key` / `Authorization` / SNMP `community` 等敏感字段。

### 4.2 架构边界

- JS 不得直连原始 socket/SSH/DB/Redis/Kafka/SNMP/本机进程；只通过 `ctx.ssh` / `ctx.mysql` / `ctx.postgres` / `ctx.docker` / `ctx.redis` / `ctx.kafka` / `ctx.http` / `ctx.command` / `ctx.snmp` / `ctx.tcp` / `ctx.udp`（及 `ctx.hosts` / `ctx.databases` / `ctx.apis` / `ctx.commands` / `ctx.snmp_devices`）。
- OpenAPI 生成 Tool 在 Go 侧经 HTTP Connector 出站，不经 Goja；磁盘 Plugin 可用 `ctx.http`（`api` 或绝对 `url`）。
- Command Connector：`ctx.command.exec` 仅执行 `commands.yaml` 白名单中的逻辑 `name`（`path` 为绝对路径数组，加载时取本机第一个可用路径），无 shell；Plugin 固定 `command` 名并将 MCP 入参映射为 `args`。
- 资源用配置中的 **name** 引用（如 `dev-ssh-111` / `local-redis` / `cmdb` / `ping`），禁止在 Plugin 里写死 IP/账号（SSH/DB/SNMP 等）。
- TCP/UDP：`ctx.tcp.exchange` / `ctx.udp.exchange` 接受调用方 `ip`/`port`（无 `net.yaml`）。**建议**在业务 Plugin 的 `main.js` 内固定或校验目标；**不建议**把 `ctx.params.ip`/`port` 原样透传给 Agent 任意拨号。
- 统一业务结果：`{ "success": true|false, "data"|"error": ... }`（见 `docs/api.md`）。
- MVP **不做**：RBAC、用户系统、Permission、Audit、allow/deny rules。

### 4.3 代码风格

- 改动保持最小、可编译；不顺手大重构、不写无关文档。
- 与现有包结构、命名、错误码保持一致。
- 公共契约变更必须同步更新对应 `docs/*`。

---

## 5. 本地测试资源（开发用）

主机与库的 **name** 如下（密码见本地 yaml，勿抄进代码）：

| 资源 name | 用途 |
|-----------|------|
| `dev-ssh-111` | SSH 测试机 `192.168.44.111` |
| `dev-ssh-112` | SSH 测试机 `192.168.44.112` |
| `local-postgres` | 本机 PostgreSQL `127.0.0.1:5432` |
| `local-redis` | 本机 Docker Redis `127.0.0.1:6379`（见 `config/redis.yaml`） |
| `local-snmp` / `local-snmp-v3` | 本机 Docker snmpsim `127.0.0.1:1161`（`make snmp-up`，见 `deploy/dev-snmp/`） |
| TCP/UDP echo | 本机 Docker echo `127.0.0.1:19090`（TCP）/ `19091`（UDP）（`make net-up`，见 `deploy/dev-net/`） |

配置入口：`config/ops-mcp.yaml`。

集成测试若依赖真实 SSH/DB：跳过条件用 build tag 或环境变量（如 `OPS_MCP_INTEGRATION=1`），默认 CI 只跑单元测试。

---

## 6. 完成定义（DoD）— 必须遵守

**任务完成 = 代码改完 + 测试全部通过 + 必要文档已同步。**  
仅「能编译」或「口头说测过了」不算完成。

### 6.1 每次改动的最低测试门禁

按改动范围执行，**全部退出码为 0** 后方可结束任务：

| 级别 | 命令 / 动作 | 适用 |
|------|-------------|------|
| 编译 | `go build ./...` | 任何 Go 改动 |
| 单元测试 | `go test ./...` | 任何 Go 改动 |
| 竞态（改并发/connector 时） | `go test -race ./...` | Connector、Runtime、并发路径 |
| 静态检查 | `go vet ./...` | 任何 Go 改动 |
| 管理 API 冒烟 | 启动服务后 `GET /`（公开 UI）、`GET /api/plugins`、`GET /api/tools`、`GET /api/hosts`、`GET /api/databases`、`GET /api/redis`、`GET /api/kafka`、`GET /api/apis`、`GET /api/commands`、`GET /api/snmp` | 影响加载/注册时 |
| TCP/UDP 联调 | `make net-up` 后对 `tcp_exchange` / `udp_exchange` 冒烟 | 改 TCP/UDP Connector / 相关 Plugin 时 |
| MCP 冒烟 | `POST /mcp`：`tools/list`；相关 `tools/call` | 影响 MCP / Plugin 时 |
| Plugin 契约 | 对应 `plugin.yml` + `main.js` 可加载且 Tool 名正确 | 新增/修改 Plugin 时 |
| 集成（可选但推荐） | 对 `dev-ssh-*` / `local-postgres` / `local-redis` 真实调用 | Connector / 相关 Plugin |

推荐一键门禁：`make check`（vet + test + build）。

### 6.2 Agent 工作流（强制）

1. 阅读本文件 + 当前 Phase 的 `docs/roadmap.md` 验收项。  
2. 实现最小改动。  
3. **运行第 6.1 节适用测试**，根据失败修复，直到通过。  
4. 若改了对外契约，更新 `docs/` 与必要时 README。  
5. 在回复中写明：执行了哪些命令、结果是否通过。  
6. **测试未通过 → 任务未完成**；不得把失败留给用户「自行再测」。

### 6.3 禁止行为

- 用 `--no-verify` 跳过钩子（除非用户明确要求）。  
- 删除/跳过失败测试以使门禁变绿（除非用户明确要求且说明原因）。  
- 提交或展示 `hosts.yaml` / `databases.yaml` / `redis.yaml` / `kafka.yaml` / `apis.yaml` / `snmp.yaml` 中的真实密钥。  
- 经 Command Connector 暴露「任意 argv」通用 Tool（须按动作拆 Plugin，且 `command` 必须在白名单内）。  
- 在未跑测试的情况下声称「已完成 / Done / 可以合并」。

---

## 7. 分阶段开发提示

**当前进度：Phase 0–9 已完成（含 Phase 9 TCP/UDP）。** 详见 [docs/roadmap.md](docs/roadmap.md)。

| Phase | 焦点 | 状态 | 完成时重点验什么 |
|-------|------|------|------------------|
| 0 | 文档 | 已完成 | 契约齐全 |
| 1 | MCP + Plugin + Goja + linux_ls/cat/grep/tail | 已完成 | `go test`；`tools/list`/`tools/call`；`/api/*` |
| 2 | SSH Connector + hosts + 其余 linux_* | 已完成 | SSH 集成；`CONNECTOR_ERROR`；日志无密码 |
| 3 | Docker Connector + docker_* | 已完成 | `docker_ps` / `docker_logs` 等端到端 |
| 4 | MySQL/PG Connector + query/version Plugin | 已完成 | parser 仅 SELECT；`limit`；`local-postgres` |
| 5 | Redis / Kafka | 已完成 | Redis/Kafka Plugin 在 `tools/list`；`/api/redis` `/api/kafka` |
| 6 | HTTP API Plugin（OpenAPI → Tool） | 已完成 | `apis.yaml`；Discovery；`tools/list` 含生成 Tool；`/api/apis` |
| 7 | Command Connector（本机进程 + `commands.yaml` 白名单） | 已完成 | 白名单；无 shell；`host_ping`；`/api/commands` |
| 8 | SNMP Connector（v2c/v3 + snmp.yaml） | 已完成 | 双模式凭据；`snmp_*` Plugin；`/api/snmp` |
| 9 | TCP / UDP Connector（私有协议） | 已完成 | `ctx.tcp`/`ctx.udp`；`tcp_exchange`/`udp_exchange`；`make net-up` |

每阶段结束输出：目录结构、启动方式、测试方式、下一阶段计划（与 roadmap 一致）。

---

## 8. 常用命令

推荐用根目录 `Makefile`（`make help` 查看全部目标）：

```bash
make check          # vet + test + build（最低门禁）
make build          # 输出 bin/ops-mcp
make test / make vet / make race
make run            # 编译并启动
make docker         # 构建镜像 ops-mcp:latest
make docker-run     # 挂载本地 config/plugins 跑容器
make tar            # 构建并打包 dist/*.tar.gz（可 GOOS/GOARCH）
make integration    # OPS_MCP_INTEGRATION=1 集成测试
```

等价裸命令：

```bash
# 编译与测试门禁
go build ./...
go vet ./...
go test ./...

# 启动
go run ./cmd/server --config ./config/ops-mcp.yaml

# 冒烟（token 与 config/ops-mcp.yaml server.auth.token 一致）
TOKEN=ops-mcp-local-dev-token
curl -s localhost:20267/health | jq .
curl -sI localhost:20267/ | head -n 5   # 嵌入 Web UI（公开）
curl -s -H "Authorization: Bearer $TOKEN" localhost:20267/api/plugins | jq .
curl -s -H "Authorization: Bearer $TOKEN" localhost:20267/api/tools | jq .
curl -s -H "Authorization: Bearer $TOKEN" localhost:20267/api/hosts | jq .
curl -s -X POST localhost:20267/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

集成测试示例：

```bash
OPS_MCP_INTEGRATION=1 go test ./internal/connector/... -count=1
```

---

## 9. 一句话提醒

**先读文档契约 → 再改代码 → 再跑通测试 → 才算任务完成。**
