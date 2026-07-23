# ops-mcp 文档

本目录描述 ops-mcp 的用户手册、架构、API、Plugin、Runtime、Connector、配置、HTTP API（OpenAPI）与开发路线图。

**当前进度：Phase 0–7 已完成（含 Phase 5 Redis / Kafka）。** 详见 [roadmap.md](roadmap.md)。

## 文档导航

| 文档 | 读者 | 内容 |
|------|------|------|
| [user-guide.md](user-guide.md) | 运维 / 使用者 / Plugin 作者 | **用户手册**：介绍、部署、内置 Plugin、自定义 Plugin |
| [architecture.md](architecture.md) | 架构师 / 贡献者 | 组件分层、请求链路、设计原则、MVP 边界 |
| [api.md](api.md) | 接入方 / 前端 | `POST /mcp`、管理接口、请求响应与错误约定 |
| [plugin.md](plugin.md) | Plugin 作者 | 目录约定、`plugin.yml` schema、Tool 注册、Plugin 清单 |
| [runtime.md](runtime.md) | Plugin 作者 | Goja `execute(ctx)` 与注入 API |
| [connector.md](connector.md) | 后端开发 | Connector 实现职责与扩展方式 |
| [configuration.md](configuration.md) | 运维 / 部署 | `ops-mcp.yaml` / `hosts.yaml` / `databases.yaml` / `redis.yaml` / `kafka.yaml` / `apis.yaml` / `commands.yaml` / `snmp.yaml` |
| [apis.md](apis.md) | 运维 / 后端 | HTTP API：`apis.yaml`、OpenAPI、Discovery、MCP Tool 生成 |
| [roadmap.md](roadmap.md) | 项目负责人 | Phase 0–7 交付物、验收标准与当前进度 |

## 快速理解路径

1. 读根目录 [README.md](../README.md) 了解定位与理念。
2. 读 [user-guide.md](user-guide.md) 完成部署、了解内置能力与扩展方式。
3. 读 [architecture.md](architecture.md) 理解整体结构。
4. 读 [plugin.md](plugin.md) + [runtime.md](runtime.md) 深入 Plugin 契约。
5. 读 [apis.md](apis.md) 了解 OpenAPI → MCP Tool。
6. 读 [api.md](api.md) 了解 Agent 与管理端如何接入。
7. 读 [roadmap.md](roadmap.md) 了解分阶段交付。

## 相关约定

- 文档以中文为主；配置键、API 路径、类型名保持英文。
- 示例中的密钥一律使用占位符（`***` / `CHANGE_ME`）。
- `main.js` / YAML 示例仅为契约说明，不可直接运行。
