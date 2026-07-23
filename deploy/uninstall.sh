#!/usr/bin/env bash
# 卸载 ops-mcp systemd 服务。
# 用法：
#   sudo ./deploy/uninstall.sh
#   sudo ./deploy/uninstall.sh --purge          # 同时删除安装目录
#   sudo ./deploy/uninstall.sh --purge --remove-user

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

APP_NAME="ops-mcp"
INSTALL_DIR="${INSTALL_DIR:-/opt/ops-mcp}"
SERVICE_NAME="${SERVICE_NAME:-ops-mcp}"
OPS_MCP_USER="${OPS_MCP_USER:-ops-mcp}"
OPS_MCP_GROUP="${OPS_MCP_GROUP:-ops-mcp}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
PURGE=0
REMOVE_USER=0

# 若存在上次安装留下的元数据，优先读取
if [[ -f "${SCRIPT_DIR}/install.env" ]]; then
  # shellcheck disable=SC1091
  source "${SCRIPT_DIR}/install.env"
fi
# 也尝试从默认安装目录读取
if [[ -f "${INSTALL_DIR}/deploy/install.env" ]]; then
  # shellcheck disable=SC1091
  source "${INSTALL_DIR}/deploy/install.env"
fi

usage() {
  cat <<EOF
Usage: sudo $0 [options]

Options:
  --install-dir DIR   安装目录（默认: ${INSTALL_DIR}）
  --purge             删除安装目录（含配置，请谨慎）
  --remove-user       删除运行用户/组（需 --purge 或服务已停）
  -h, --help          显示帮助
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    --purge)
      PURGE=1
      shift
      ;;
    --remove-user)
      REMOVE_USER=1
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

UNIT_DST="${SYSTEMD_DIR}/${SERVICE_NAME}.service"

echo "==> 卸载 ${APP_NAME}"
echo "    服务名: ${SERVICE_NAME}"
echo "    安装目录: ${INSTALL_DIR}"

if systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1 \
  || [[ -f "${UNIT_DST}" ]]; then
  systemctl stop "${SERVICE_NAME}.service" 2>/dev/null || true
  systemctl disable "${SERVICE_NAME}.service" 2>/dev/null || true
fi

if [[ -f "${UNIT_DST}" ]]; then
  rm -f "${UNIT_DST}"
  systemctl daemon-reload
  systemctl reset-failed "${SERVICE_NAME}.service" 2>/dev/null || true
  echo "    已移除 unit: ${UNIT_DST}"
fi

if [[ "${PURGE}" -eq 1 ]]; then
  if [[ -d "${INSTALL_DIR}" ]]; then
    rm -rf "${INSTALL_DIR}"
    echo "    已删除目录: ${INSTALL_DIR}"
  fi
else
  echo "    保留安装目录: ${INSTALL_DIR}"
  echo "    （如需删除请加 --purge）"
fi

if [[ "${REMOVE_USER}" -eq 1 ]]; then
  if id -u "${OPS_MCP_USER}" >/dev/null 2>&1; then
    userdel "${OPS_MCP_USER}" 2>/dev/null || true
    echo "    已删除用户: ${OPS_MCP_USER}"
  fi
  if getent group "${OPS_MCP_GROUP}" >/dev/null 2>&1; then
    groupdel "${OPS_MCP_GROUP}" 2>/dev/null || true
    echo "    已删除组: ${OPS_MCP_GROUP}"
  fi
fi

echo "卸载完成。"
