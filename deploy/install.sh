#!/usr/bin/env bash
# 将解压后的 ops-mcp 发行包安装为 systemd 服务。
# 用法：
#   sudo ./deploy/install.sh
#   sudo INSTALL_DIR=/opt/ops-mcp ./deploy/install.sh
#   sudo ./deploy/install.sh --no-start

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

APP_NAME="ops-mcp"
INSTALL_DIR="${INSTALL_DIR:-/opt/ops-mcp}"
SERVICE_NAME="${SERVICE_NAME:-ops-mcp}"
OPS_MCP_USER="${OPS_MCP_USER:-ops-mcp}"
OPS_MCP_GROUP="${OPS_MCP_GROUP:-ops-mcp}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
START_SERVICE=1

usage() {
  cat <<EOF
Usage: sudo $0 [options]

Options:
  --install-dir DIR   安装目录（默认: ${INSTALL_DIR}）
  --user USER         运行用户（默认: ${OPS_MCP_USER}）
  --group GROUP       运行组（默认: ${OPS_MCP_GROUP}）
  --no-start          安装后不立即启动
  -h, --help          显示帮助

环境变量：INSTALL_DIR / OPS_MCP_USER / OPS_MCP_GROUP / SERVICE_NAME
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    --user)
      OPS_MCP_USER="$2"
      shift 2
      ;;
    --group)
      OPS_MCP_GROUP="$2"
      shift 2
      ;;
    --no-start)
      START_SERVICE=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$(id -u)" -ne 0 ]]; then
  echo "请使用 root 执行：sudo $0" >&2
  exit 1
fi

BINARY_SRC="${PACKAGE_ROOT}/bin/${APP_NAME}"
if [[ ! -x "${BINARY_SRC}" ]]; then
  echo "找不到可执行文件: ${BINARY_SRC}" >&2
  echo "请先 make tar / make build，或使用完整发行包。" >&2
  exit 1
fi

if [[ ! -d "${PACKAGE_ROOT}/plugins" ]]; then
  echo "找不到 plugins 目录: ${PACKAGE_ROOT}/plugins" >&2
  exit 1
fi

if [[ ! -f "${SCRIPT_DIR}/${APP_NAME}.service" ]]; then
  echo "找不到 unit 文件: ${SCRIPT_DIR}/${APP_NAME}.service" >&2
  exit 1
fi

echo "==> 安装 ${APP_NAME}"
echo "    安装目录: ${INSTALL_DIR}"
echo "    运行用户: ${OPS_MCP_USER}:${OPS_MCP_GROUP}"

if ! getent group "${OPS_MCP_GROUP}" >/dev/null 2>&1; then
  groupadd --system "${OPS_MCP_GROUP}"
fi
if ! id -u "${OPS_MCP_USER}" >/dev/null 2>&1; then
  NOLOGIN="$(command -v nologin || true)"
  NOLOGIN="${NOLOGIN:-/usr/sbin/nologin}"
  useradd --system --gid "${OPS_MCP_GROUP}" --home-dir "${INSTALL_DIR}" \
    --no-create-home --shell "${NOLOGIN}" --comment "ops-mcp service" "${OPS_MCP_USER}"
fi

mkdir -p "${INSTALL_DIR}/bin" "${INSTALL_DIR}/config" "${INSTALL_DIR}/plugins"

install -m 0755 "${BINARY_SRC}" "${INSTALL_DIR}/bin/${APP_NAME}"

# 同步 plugins（覆盖代码，不删用户额外文件以外的整树替换）
rm -rf "${INSTALL_DIR}/plugins"
mkdir -p "${INSTALL_DIR}/plugins"
cp -a "${PACKAGE_ROOT}/plugins/." "${INSTALL_DIR}/plugins/"

# 同步 OpenAPI 文档（只读契约，每次安装覆盖；与 make tar 打包内容一致）
OPENAPI_SRC="${PACKAGE_ROOT}/config/openapi"
OPENAPI_DST="${INSTALL_DIR}/config/openapi"
if [[ -d "${OPENAPI_SRC}" ]]; then
  mkdir -p "${OPENAPI_DST}"
  cp -a "${OPENAPI_SRC}/." "${OPENAPI_DST}/"
  echo "    已同步 OpenAPI: ${OPENAPI_DST}"
else
  echo "警告: 发行包缺少 config/openapi，跳过同步（apis.yaml 中的 openapi.path 可能不可用）" >&2
fi

# 配置：仅在目标不存在时从 example 生成，避免覆盖已有凭据
copy_config_if_missing() {
  local name="$1"
  local dest="${INSTALL_DIR}/config/${name}"
  local src_example="${PACKAGE_ROOT}/config/${name}.example"
  local src_plain="${PACKAGE_ROOT}/config/${name}"

  if [[ -f "${dest}" ]]; then
    echo "    保留已有配置: ${dest}"
    return
  fi
  if [[ -f "${src_example}" ]]; then
    install -m 0640 "${src_example}" "${dest}"
  elif [[ -f "${src_plain}" ]]; then
    install -m 0640 "${src_plain}" "${dest}"
  else
    echo "警告: 缺少配置模板 ${name}" >&2
    return
  fi
  echo "    已生成配置: ${dest}（请按需修改）"
}

copy_config_if_missing "ops-mcp.yaml"
copy_config_if_missing "hosts.yaml"
copy_config_if_missing "databases.yaml"
copy_config_if_missing "apis.yaml"

# 将主配置中的相对路径改写为安装目录绝对路径
OPS_YAML="${INSTALL_DIR}/config/ops-mcp.yaml"
if [[ -f "${OPS_YAML}" ]]; then
  # 兼容 ./plugins 与 "plugins" 等写法
  sed -i.bak \
    -e "s|dir: *\"\./plugins\"|dir: \"${INSTALL_DIR}/plugins\"|g" \
    -e "s|dir: *\./plugins|dir: \"${INSTALL_DIR}/plugins\"|g" \
    -e "s|hosts: *\"\./config/hosts.yaml\"|hosts: \"${INSTALL_DIR}/config/hosts.yaml\"|g" \
    -e "s|hosts: *\./config/hosts.yaml|hosts: \"${INSTALL_DIR}/config/hosts.yaml\"|g" \
    -e "s|databases: *\"\./config/databases.yaml\"|databases: \"${INSTALL_DIR}/config/databases.yaml\"|g" \
    -e "s|databases: *\./config/databases.yaml|databases: \"${INSTALL_DIR}/config/databases.yaml\"|g" \
    "${OPS_YAML}"
  rm -f "${OPS_YAML}.bak"
fi

chown -R "${OPS_MCP_USER}:${OPS_MCP_GROUP}" "${INSTALL_DIR}"
chmod 0750 "${INSTALL_DIR}/config"
chmod 0750 "${INSTALL_DIR}/config/openapi" 2>/dev/null || true
chmod 0640 "${INSTALL_DIR}/config/"*.yaml 2>/dev/null || true
chmod 0640 "${INSTALL_DIR}/config/openapi/"*.yaml 2>/dev/null || true

UNIT_DST="${SYSTEMD_DIR}/${SERVICE_NAME}.service"
sed \
  -e "s|__INSTALL_DIR__|${INSTALL_DIR}|g" \
  -e "s|__OPS_MCP_USER__|${OPS_MCP_USER}|g" \
  -e "s|__OPS_MCP_GROUP__|${OPS_MCP_GROUP}|g" \
  "${SCRIPT_DIR}/${APP_NAME}.service" > "${UNIT_DST}"
chmod 0644 "${UNIT_DST}"

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}.service"

if [[ "${START_SERVICE}" -eq 1 ]]; then
  systemctl restart "${SERVICE_NAME}.service"
  systemctl --no-pager --full status "${SERVICE_NAME}.service" || true
fi

cat <<EOF

安装完成。

  配置目录: ${INSTALL_DIR}/config
  OpenAPI:  ${INSTALL_DIR}/config/openapi
  请编辑 hosts.yaml / ops-mcp.yaml / apis.yaml（token 等）后如有改动可执行:
    sudo systemctl restart ${SERVICE_NAME}

常用命令:
  sudo systemctl start ${SERVICE_NAME}
  sudo systemctl stop ${SERVICE_NAME}
  sudo systemctl restart ${SERVICE_NAME}
  sudo systemctl status ${SERVICE_NAME}
  sudo journalctl -u ${SERVICE_NAME} -f

卸载:
  sudo ${INSTALL_DIR}/deploy/uninstall.sh
  # 或使用发行包内: sudo ./deploy/uninstall.sh --purge
EOF

# 把 uninstall 脚本也拷到安装目录，方便日后卸载
mkdir -p "${INSTALL_DIR}/deploy"
install -m 0755 "${SCRIPT_DIR}/uninstall.sh" "${INSTALL_DIR}/deploy/uninstall.sh"
install -m 0644 "${SCRIPT_DIR}/${APP_NAME}.service" "${INSTALL_DIR}/deploy/${APP_NAME}.service"
# 记录安装元数据供 uninstall 使用
cat > "${INSTALL_DIR}/deploy/install.env" <<EOF
INSTALL_DIR=${INSTALL_DIR}
SERVICE_NAME=${SERVICE_NAME}
OPS_MCP_USER=${OPS_MCP_USER}
OPS_MCP_GROUP=${OPS_MCP_GROUP}
SYSTEMD_DIR=${SYSTEMD_DIR}
EOF
chown -R "${OPS_MCP_USER}:${OPS_MCP_GROUP}" "${INSTALL_DIR}/deploy"
