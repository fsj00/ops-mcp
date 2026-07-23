# ops-mcp 部署（systemd）

## 发行包安装

在构建机：

```bash
make tar
# 交叉编译到 Linux 服务器：
make tar GOOS=linux GOARCH=amd64
```

将 `dist/ops-mcp-*.tar.gz` 拷到目标机后：

```bash
tar -xzf ops-mcp-*-linux-amd64.tar.gz
cd ops-mcp-*-linux-amd64
sudo ./deploy/install.sh
```

安装后编辑配置（首次会从 example 生成；`config/openapi/` 每次安装同步覆盖）：

```bash
sudo vi /opt/ops-mcp/config/ops-mcp.yaml
sudo vi /opt/ops-mcp/config/hosts.yaml
sudo vi /opt/ops-mcp/config/apis.yaml   # 可选；OpenAPI → MCP Tool
sudo systemctl restart ops-mcp
```

## 常用命令

```bash
sudo systemctl start ops-mcp
sudo systemctl stop ops-mcp
sudo systemctl restart ops-mcp
sudo systemctl status ops-mcp
sudo journalctl -u ops-mcp -f
```

## 卸载

```bash
sudo /opt/ops-mcp/deploy/uninstall.sh           # 停服务，保留文件
sudo /opt/ops-mcp/deploy/uninstall.sh --purge   # 连同安装目录删除
sudo ./deploy/uninstall.sh --purge --remove-user
```

## 自定义

```bash
sudo INSTALL_DIR=/usr/local/ops-mcp ./deploy/install.sh
sudo ./deploy/install.sh --user ops-mcp --no-start
```
