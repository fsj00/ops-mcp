# ops-mcp

面向 AI Agent 的 Remote MCP Ops 平台。

Claude Code、Codex、ChatGPT Agent 等可通过 **HTTP Remote MCP API** 连接 ops-mcp，以 MCP Tool 形式调用 Linux 运维、SSH 远程执行、Docker / 数据库 / Redis / Kafka 查询、基于 OpenAPI 的 HTTP API、经 Command Connector 调用本机白名单 CLI（如 `ping`）、经 SNMP Connector 查询交换机，以及经 TCP/UDP Connector 对接私有字节协议设备等能力。

## 核心理念

**能力即可见 Tool。**

- 不创建磁盘 Plugin、或不通过 Discovery 暴露 OpenAPI Operation，则不存在该能力。
- 磁盘能力对应独立 Plugin（如 `linux_ls`、`docker_ps`、`redis_ping`）；HTTP API 可由 `apis.yaml` + OpenAPI 自动生成 Tool。
- 磁盘 Plugin 由 `plugin.yml` + `main.js` 组成；Go 负责协议、加载、校验与连接器，JavaScript（Goja）负责编排与调用。OpenAPI Tool 不经 Goja。

## 当前状态

**Phase 0–9 已完成**（含 Phase 9 TCP/UDP）。详见 [docs/roadmap.md](docs/roadmap.md)。

| 纳入 | 暂不实现 |
|------|----------|
| MCP Remote Server（`POST /mcp`） | RBAC / 用户系统 |
| Plugin Framework + Goja | Permission / Audit |
| SSH Connector（`hosts.yaml`） | allow_rules / deny_rules |
| Docker Connector（经 SSH） | — |
| MySQL / PostgreSQL / Redis / Kafka Connector + Plugin | — |
| Linux + Docker + DB + Redis + Kafka 基础 Plugin | — |
| HTTP API：`apis.yaml` + OpenAPI → MCP Tool（Phase 6） | ops-mcp 自身 Swagger UI |
| Command Connector：`commands.yaml` 白名单 + 本机 CLI Plugin（Phase 7） | — |
| SNMP Connector：`snmp.yaml` + get/walk/bulk / sysinfo / interfaces（Phase 8） | — |
| TCP / UDP Connector：`ctx.tcp` / `ctx.udp` + 冒烟 Plugin（Phase 9） | — |

## 快速开始

```bash
# 1. 复制并填写真实凭据（勿提交）
cp config/hosts.yaml.example config/hosts.yaml
cp config/databases.yaml.example config/databases.yaml
cp config/redis.yaml.example config/redis.yaml
cp config/kafka.yaml.example config/kafka.yaml  # 可选；Kafka 只读查询
cp config/apis.yaml.example config/apis.yaml   # 可选；OpenAPI → MCP Tool
# config/commands.yaml 已自带常用本机 CLI 白名单（ping/dig/…）；可按环境改 path 数组
cp config/snmp.yaml.example config/snmp.yaml     # 可选；交换机 SNMP
make snmp-up                                     # 可选；本地 SNMP 模拟器 UDP 1161（deploy/dev-snmp）
make net-up                                      # 可选；本地 TCP/UDP echo 19090/19091（deploy/dev-net）

# 2. 编译与测试
make check

# 3. 启动（开发）
make run
```

### 生产安装（systemd）

```bash
make tar GOOS=linux GOARCH=amd64
# 将 dist/ops-mcp-*.tar.gz 拷到服务器后：
tar -xzf ops-mcp-*-linux-amd64.tar.gz
cd ops-mcp-*-linux-amd64
sudo ./deploy/install.sh
```

详见 [docs/user-guide.md](docs/user-guide.md) 与 [deploy/README.md](deploy/README.md)。

默认监听见 `config/ops-mcp.yaml`（本机开发常用 `20267`）。

浏览器打开 `http://localhost:20267/` 可查看当前 Tools 与资源配置（静态页公开；页面内输入与 `server.auth.token` 一致的 token 后加载 `/api/*`）。

### 冒烟

```bash
TOKEN=ops-mcp-local-dev-token   # 与 config/ops-mcp.yaml server.auth.token 一致

curl -s localhost:20267/health | jq .
curl -s -H "Authorization: Bearer $TOKEN" localhost:20267/api/plugins | jq .
curl -s -X POST localhost:20267/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

### Claude Code 接入

在任意项目目录创建 `.mcp.json`（须带 token）：

```json
{
  "mcpServers": {
    "ops-mcp": {
      "type": "http",
      "url": "http://127.0.0.1:20267/mcp",
      "headers": {
        "Authorization": "Bearer ops-mcp-local-dev-token"
      }
    }
  }
}
```

也可用环境变量：`Authorization: Bearer ${OPS_MCP_AUTH_TOKEN}`，并与服务端 `OPS_MCP_AUTH_TOKEN` / `server.auth.token` 对齐。

调用示例：

- `list_hosts`：无需参数
- `docker_ps`：`host=dev-ssh-111` / `dev-ssh-112`
- `linux_top`：`host=dev-ssh-111`，`count=10`
- `redis_ping`：`redis=local-redis`

## 目录结构

```
ops-mcp/
├── cmd/server/
├── internal/
│   ├── api/
│   ├── mcp/
│   ├── plugin/
│   ├── runtime/
│   ├── executor/
│   ├── connector/{ssh,docker,mysql,postgres,redis,http,dbutil,sqlguard}/
│   ├── openapi/          # Phase 6：OpenAPI / Discovery / Tool 生成
│   ├── config/
│   └── model/
├── plugins/{linux,docker,mysql,postgres,redis,hosts,databases,apis}/
├── config/
├── deploy/
└── docs/
```

## 文档

| 文档 | 说明 |
|------|------|
| [docs/user-guide.md](docs/user-guide.md) | **用户手册**（介绍 / 部署 / 内置 Plugin / 自定义 Plugin） |
| [cursor.md](cursor.md) | Agent 开发上下文与测试完成门禁 |
| [docs/architecture.md](docs/architecture.md) | 架构 |
| [docs/api.md](docs/api.md) | HTTP / MCP API |
| [docs/plugin.md](docs/plugin.md) | Plugin 框架 |
| [docs/runtime.md](docs/runtime.md) | Goja ctx API |
| [docs/connector.md](docs/connector.md) | Connector |
| [docs/configuration.md](docs/configuration.md) | 配置 |
| [docs/apis.md](docs/apis.md) | HTTP API：`apis.yaml` + OpenAPI → MCP Tool |
| [docs/roadmap.md](docs/roadmap.md) | 路线图 |

## 许可证

[Apache License 2.0](LICENSE)
