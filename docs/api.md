# HTTP API

ops-mcp 对外提供两类 HTTP 接口：

1. **MCP 端点** — AI Agent 使用（JSON-RPC over HTTP）
2. **管理端点** — 运维与调试使用（REST）

> 状态：与当前实现一致；`GET /api/apis` 与 OpenAPI Tool 合并计数为 Phase 6 契约（实现见 [apis.md](apis.md) / [roadmap.md](roadmap.md)）。契约变更须同步改代码与本文档。

## 约定

| 项 | 说明 |
|----|------|
| Base URL | 默认 `http://<host>:20267`（端口由 `ops-mcp.yaml` 配置） |
| Content-Type | `application/json` |
| 字符编码 | UTF-8 |

### 统一业务结果（Tool 执行）

Plugin / Tool 执行成功：

```json
{
  "success": true,
  "data": {}
}
```

失败：

```json
{
  "success": false,
  "error": {
    "code": "PLUGIN_TIMEOUT",
    "message": "plugin linux_ls exceeded timeout 10s"
  }
}
```

常见 `error.code`：

| code | 含义 |
|------|------|
| `PLUGIN_NOT_FOUND` | Tool / Plugin 不存在 |
| `INVALID_PARAMS` | 参数不符合 `plugin.yml` input |
| `PLUGIN_TIMEOUT` | 超过 `timeout` |
| `CONNECTOR_ERROR` | SSH / DB / Docker / Redis / HTTP / Command（本机进程）调用失败 |
| `RUNTIME_ERROR` | Goja 执行异常 |
| `INTERNAL_ERROR` | 未分类内部错误 |

---

## MCP：`POST /mcp`

JSON-RPC 2.0 over HTTP。请求体为**单个** JSON-RPC 对象（不支持 batch）。

### 通用请求壳

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "<method>",
  "params": {}
}
```

### 与 Plugin 的关系

| MCP 概念 | ops-mcp 映射 |
|----------|----------------|
| Tool name | `plugin.yml` 的 `name` |
| Tool description | `plugin.yml` 的 `description` |
| Tool inputSchema | 由 `plugin.yml` 的 `input` 生成 |
| tools/call 参数 | 传入 Goja `ctx.params` |

### `initialize`

Agent 握手时调用，返回协议版本与 server 能力。成功时响应头可带 `Mcp-Session-Id`（Streamable HTTP 会话标识）。

**请求示例：**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "claude-code",
      "version": "1.0"
    }
  }
}
```

**响应要点：** `serverInfo.name` = `ops-mcp`，声明支持 tools。

### `tools/list`

返回当前已加载 Plugin 对应的 Tool 列表。

**请求：**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list",
  "params": {}
}
```

**响应示例：**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "linux_ls",
        "description": "Linux目录列表",
        "inputSchema": {
          "type": "object",
          "properties": {
            "path": {
              "type": "string"
            },
            "host": {
              "type": "string",
              "description": "hosts.yaml 中的主机名"
            }
          },
          "required": ["path"]
        }
      }
    ]
  }
}
```

### `tools/call`

执行指定 Tool（Plugin）。

**请求：**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "linux_ls",
    "arguments": {
      "path": "/var/log",
      "host": "prod-web-01"
    }
  }
}
```

**成功响应（示意）：**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"success\":true,\"data\":{\"stdout\":\"...\",\"stderr\":\"\",\"exit_code\":0}}"
      }
    ],
    "isError": false
  }
}
```

> 业务结果（`{success, data|error}`）序列化为 JSON 字符串，放在 MCP `content[0].text`；`isError` 与 `success` 取反对应。Agent 应解析 `text` 得到业务层结果。

**失败时：** 仍返回 HTTP 200 + `result.isError = true`（业务错误码见上文）；非法 JSON-RPC 等方法级错误走 JSON-RPC `error`。

---

## 管理接口

### `GET /`（Web UI）

嵌入式浏览器控制台（静态页，**无需鉴权**）。页面内输入 token 后调用下方只读 `/api/*` 查看 Tools 与资源配置。

| 项 | 说明 |
|----|------|
| 路径 | `GET /`、`GET /index.html` |
| 鉴权 | 公开（与 `GET /health` 相同） |
| 内容 | `text/html`（`web/` 经 `go:embed` 打入二进制） |

### `GET /api/plugins`

返回已加载 Plugin 元数据列表。

**响应示例：**

```json
{
  "plugins": [
    {
      "name": "linux_ls",
      "version": "1.0",
      "description": "Linux目录列表",
      "type": "command",
      "target": {
        "type": "ssh"
      },
      "runtime": "javascript",
      "timeout": "10s",
      "path": "plugins/linux/ls"
    }
  ]
}
```

### `GET /api/tools`

返回当前 MCP Tool 列表（与 `tools/list` 信息等价，便于运维直接 HTTP 查看）。包含磁盘 Plugin 与 OpenAPI 生成 Tool。

**响应示例：**

```json
{
  "tools": [
    {
      "name": "linux_ls",
      "description": "Linux目录列表",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": { "type": "string" }
        },
        "required": ["path"]
      }
    }
  ]
}
```

### `GET /api/hosts`

返回 `hosts.yaml` 中的主机清单（**不含密码 / 私钥**）。

**响应示例：**

```json
{
  "count": 1,
  "hosts": [
    {
      "name": "dev-ssh-111",
      "description": "SSH 测试机",
      "labels": {
        "env": "dev"
      },
      "address": {
        "host": "192.168.44.111",
        "port": 22
      },
      "auth_type": "password",
      "username": "root"
    }
  ]
}
```

### `GET /api/databases`

返回 `databases.yaml` 中的数据库清单（**不含密码**）。

**响应示例：**

```json
{
  "count": 1,
  "databases": [
    {
      "name": "local-postgres",
      "description": "local postgres (dev)",
      "labels": {
        "env": "dev"
      },
      "type": "postgresql",
      "connection": {
        "host": "127.0.0.1",
        "port": 5432,
        "username": "postgres",
        "database": "postgres",
        "sslmode": "disable"
      },
      "readonly": true,
      "limit": 1000
    }
  ]
}
```

### `GET /api/redis`

返回 `redis.yaml` 中的 Redis 实例清单（**不含密码**）。

**响应示例：**

```json
{
  "count": 1,
  "redis": [
    {
      "name": "local-redis",
      "description": "local docker redis",
      "labels": {
        "env": "dev"
      },
      "connection": {
        "host": "127.0.0.1",
        "port": 6379,
        "username": "",
        "tls": {
          "enabled": false,
          "has_ca": false,
          "has_client_cert": false
        }
      },
      "readonly": true,
      "limit": 1000
    }
  ]
}
```

### `GET /api/kafka`

返回 `kafka.yaml` 中的 Kafka 集群清单（**不含密码 / PEM**）。

**响应示例：**

```json
{
  "count": 1,
  "kafka": [
    {
      "name": "local-kafka",
      "description": "local kafka (dev)",
      "labels": {
        "env": "dev"
      },
      "connection": {
        "brokers": ["127.0.0.1:9092"],
        "sasl": {
          "enabled": false,
          "mechanism": "",
          "username": ""
        },
        "tls": {
          "enabled": false,
          "has_ca": false,
          "has_client_cert": false
        }
      },
      "readonly": true,
      "limit": 1000
    }
  ]
}
```

### `GET /api/apis`

返回 `apis.yaml` 中的 HTTP API 服务清单（**不含 headers 原文**）。完整契约见 [apis.md](apis.md)。

**响应示例：**

```json
{
  "count": 1,
  "apis": [
    {
      "name": "cmdb",
      "description": "CMDB API Service",
      "labels": {
        "env": "production",
        "system": "cmdb"
      },
      "base_url": "http://cmdb-api:8080",
      "prefix": "cmdb_",
      "openapi_path": "./openapi/cmdb.yaml",
      "timeout": "10s",
      "verify_tls": true,
      "has_headers": true,
      "tool_count": 12
    }
  ]
}
```

### `GET /api/commands`

返回 `commands.yaml` 中的本机命令白名单摘要。

**响应示例：**

```json
{
  "count": 1,
  "commands": [
    {
      "name": "ping",
      "description": "本机 ping（次数与目标由 Plugin 约束）",
      "path": "/sbin/ping"
    }
  ]
}
```

`path` 为加载时从 `commands.yaml` 路径数组中解析出的本机可用绝对路径；若某条目全部路径不可用则不会出现在列表中（启动 / reload 时会打 warning）。

### `GET /api/snmp`

返回 `snmp.yaml` 中的 SNMP 设备清单（**不含密钥**）。支持查询参数：

| 参数 | 说明 |
|------|------|
| `label` | 可重复：`label=site=dc1` |
| `labels` | 逗号分隔：`labels=site=dc1,role=core` |
| `limit` | 返回条数，缺省 100 |
| `offset` | 分页偏移 |

**响应示例：**

```json
{
  "count": 1,
  "devices": [
    {
      "name": "sw-dc1-core-01",
      "description": "DC1 core switch",
      "address": { "host": "10.10.0.1", "port": 161 },
      "auth_mode": "credential",
      "credential": "dc1-ro-v2c",
      "labels": { "site": "dc1", "role": "core" }
    }
  ]
}
```

### `POST /api/reload`

重新扫描并加载磁盘 Plugin，并按需重建 OpenAPI Tools（不重启进程）。请求体省略时默认只重载磁盘 Plugin；`config: true` 时同时重载 `hosts.yaml` / `databases.yaml` / `redis.yaml` / `kafka.yaml` / `apis.yaml` / `commands.yaml` / `snmp.yaml`，并重建 OpenAPI 生成的 MCP Tools。

**请求体（可选）：**

```json
{
  "plugins": true,
  "config": false
}
```

**响应示例：**

```json
{
  "reloaded": true,
  "plugins_count": 63,
  "tools_count": 75
}
```

`tools_count` = 磁盘 Plugin 数 + OpenAPI 生成 Tool 数。`plugins_count` 仅计磁盘 Plugin。

整体加载失败（例如 0 个磁盘 Plugin 成功，或磁盘 Plugin / OpenAPI Tool 重名冲突，或 `apis.yaml` / OpenAPI 解析失败）时返回 HTTP 500，并**保留**上一份已加载集。

---

## 后续扩展（非 MVP）

以下**不作为当前契约**：

- `GET /ready`
- **ops-mcp 自身**的 OpenAPI / Swagger UI（上游 API 的 OpenAPI 消费见 [apis.md](apis.md)）
- mTLS / OAuth（服务端 HTTP）
- Metrics（Prometheus）

## 错误与 HTTP 状态码

| 场景 | HTTP | 说明 |
|------|------|------|
| 缺少/错误 auth token | 401 | `server.auth.token` 已配置时 |
| MCP JSON-RPC 业务失败 | 200 | 错误放在 JSON-RPC `error` 或 Tool `isError` |
| 请求体非法 JSON | 400 | |
| 管理接口资源不存在 | 404 | |
| 未实现方法 | 405 / JSON-RPC method not found | |
| 服务内部异常 | 500 | |

### 鉴权

当 `config/ops-mcp.yaml` 中 `server.auth.token` 非空时，除公开路径外所有接口需携带：

```http
Authorization: Bearer <token>
```

或 `X-API-Key: <token>`，或查询参数 `?token=`。环境变量 `OPS_MCP_AUTH_TOKEN` 可覆盖配置文件中的 token。

**公开路径（无需 token）：** `GET /health`、`GET /`、`GET /index.html`。`/api/*` 与 `/mcp` 仍需鉴权。
