#!/bin/sh

# ==============================================================================
# imageapi 服务安装脚本 for POSIX sh (兼容 Debian/Ubuntu)
# ==============================================================================

# -- 脚本配置 --
SERVICE_NAME="imageapi"
GITHUB_REPO="zouzonghao/imageapi"
INSTALL_DIR="/opt/${SERVICE_NAME}"
EXECUTABLE_NAME="imageapi"
SERVICE_FILE_PATH="/etc/systemd/system/${SERVICE_NAME}.service"

# -- 颜色定义 (自动检测终端) --
if [ -t 1 ]; then
    GREEN='\033[0;32m'
    RED='\033[0;31m'
    YELLOW='\033[0;33m'
    NC='\033[0m'
else
    GREEN=''
    RED=''
    YELLOW=''
    NC=''
fi

# -- 函数定义 --

die() {
    printf "${RED}错误: %s${NC}\n" "$1" >&2
    exit 1
}

info() {
    printf "${GREEN}信息: %s${NC}\n" "$1"
}

warn() {
    printf "${YELLOW}警告: %s${NC}\n" "$1"
}

# -- 检查操作系统 --
check_os() {
    if [ -f /etc/os-release ]; then
        # shellcheck source=/dev/null
        . /etc/os-release
        if [ "$ID" = "ubuntu" ] || [ "$ID" = "debian" ] || echo "$ID_LIKE" | grep -q "debian"; then
            info "检测到兼容的操作系统: $PRETTY_NAME"
            return
        fi
    fi
    die "此脚本仅支持 Debian 或 Ubuntu 系统。"
}

# 安装或更新函数
install_service() {
    info "开始安装或更新 ${SERVICE_NAME} 服务..."

    # -- 动态获取最新版本信息 --
    info "正在从 ${GITHUB_REPO} 获取最新版本信息..."
    API_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
    
    # 使用 curl 获取最新 release 信息
    API_RESPONSE=$(curl -s "$API_URL")
    
    # 从 API 响应中提取 tag_name 和 browser_download_url
    LATEST_VERSION=$(echo "$API_RESPONSE" | grep '"tag_name":' | cut -d '"' -f 4)
    DOWNLOAD_URL=$(echo "$API_RESPONSE" | grep "browser_download_url" | grep "${SERVICE_NAME}-linux-amd64.tar.gz" | cut -d '"' -f 4)

    if [ -z "$LATEST_VERSION" ]; then
        die "无法获取最新的版本号。请检查仓库地址或网络连接。"
    fi
    if [ -z "$DOWNLOAD_URL" ]; then
        die "在最新版本 ${LATEST_VERSION} 中未找到名为 ${SERVICE_NAME}-linux-amd64.tar.gz 的产物。"
    fi
    info "找到最新版本: ${LATEST_VERSION}"

    # -- 检查是否已安装 --
    if [ -f "$SERVICE_FILE_PATH" ]; then
        warn "${SERVICE_NAME} 服务似乎已经安装。"
        printf "是否要覆盖安装? (y/n): "
        read -r REPLY
        case "$REPLY" in
            [Yy]*) 
                info "正在停止服务以进行覆盖安装..."
                systemctl stop "${SERVICE_NAME}" >/dev/null 2>&1 || true
                ;;
            *) 
                info "安装已取消。"
                exit 0 
                ;;
        esac
    fi

    # -- 配置文件处理 --
    CONFIG_FILE="${INSTALL_DIR}/config.json"
    if [ -f "$CONFIG_FILE" ]; then
        printf "是否要保留现有的配置文件 (${CONFIG_FILE})? (y/n): "
        read -r REPLY
        case "$REPLY" in
            [Yy]*) 
                info "将保留现有配置文件。"
                # 备份配置文件
                cp "$CONFIG_FILE" "/tmp/config.json.bak"
                ;;
            *) 
                info "现有配置文件将被删除。"
                rm -f "$CONFIG_FILE"
                ;;
        esac
    fi

    # -- 图片数据处理 --
    IMAGES_DIR="${INSTALL_DIR}/images"
    if [ -d "$IMAGES_DIR" ]; then
        printf "检测到现有的图片目录 (${IMAGES_DIR})。是否要删除? (警告: 这将删除所有已生成的图片) (y/n): "
        read -r REPLY
        case "$REPLY" in
            [Yy]*)
                info "将删除现有图片目录..."
                rm -rf "$IMAGES_DIR" || die "删除图片目录失败。"
                ;;
            *)
                info "将保留现有图片目录。"
                # 备份图片目录
                mv "$IMAGES_DIR" "/tmp/images.bak"
                ;;
        esac
    fi

    command -v curl >/dev/null 2>&1 || die "需要 curl 命令来下载文件，请先安装 (sudo apt install curl)。"
    command -v tar >/dev/null 2>&1 || die "需要 tar 命令来解压文件，请先安装 (sudo apt install tar)。"

    info "创建安装目录: ${INSTALL_DIR}"
    mkdir -p "$INSTALL_DIR" || die "无法创建安装目录 ${INSTALL_DIR}。"

    TEMP_FILE="/tmp/${SERVICE_NAME}-download.tar.gz"
    info "从 ${DOWNLOAD_URL} 下载文件..."
    curl -L -o "$TEMP_FILE" "$DOWNLOAD_URL" || die "下载文件失败。"

    info "清空旧文件并解压新文件到 ${INSTALL_DIR}..."
    # 清理目录，但保留 config.json
    find "$INSTALL_DIR" -mindepth 1 ! -name 'config.json' -exec rm -rf {} +
    tar -xzf "$TEMP_FILE" -C "$INSTALL_DIR" || die "解压文件失败。"
    
    # 如果之前备份了，就恢复
    if [ -f "/tmp/config.json.bak" ]; then
        mv "/tmp/config.json.bak" "$CONFIG_FILE"
    fi
    if [ -d "/tmp/images.bak" ]; then
        mv "/tmp/images.bak" "$IMAGES_DIR"
    fi

    if [ ! -f "${INSTALL_DIR}/${EXECUTABLE_NAME}" ]; then
        die "解压后未找到预期的可执行文件: ${EXECUTABLE_NAME}"
    fi

    info "设置执行权限..."
    chmod +x "${INSTALL_DIR}/${EXECUTABLE_NAME}" || die "设置执行权限失败。"

    info "创建或更新 systemd 服务文件..."
    cat << EOF > "$SERVICE_FILE_PATH"
[Unit]
Description=${SERVICE_NAME} Service
After=network.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${EXECUTABLE_NAME}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    
    info "启用并启动 ${SERVICE_NAME} 服务..."
    systemctl enable "${SERVICE_NAME}"
    systemctl start "${SERVICE_NAME}"
    
    # -- 记录版本号 --
    echo "$LATEST_VERSION" > "${INSTALL_DIR}/.version" || warn "无法写入版本文件。"
    
    rm -f "$TEMP_FILE"

    info "----------------------------------------------------"
    printf "${GREEN}操作成功! (${LATEST_VERSION})${NC}\n"
    printf "服务状态检查: ${YELLOW}sudo systemctl status ${SERVICE_NAME}${NC}\n"
    printf "查看服务日志: ${YELLOW}sudo journalctl -u ${SERVICE_NAME} -f${NC}\n"
    info "----------------------------------------------------"
}

# 卸载函数
uninstall_service() {
    info "开始卸载 ${SERVICE_NAME} 服务..."
    if [ ! -f "$SERVICE_FILE_PATH" ]; then
        warn "${SERVICE_NAME} 服务未安装。"
        return
    fi

    # -- 配置文件处理 --
    CONFIG_FILE="${INSTALL_DIR}/config.json"
    DELETE_CONFIG=1
    if [ -f "$CONFIG_FILE" ]; then
        printf "是否要删除配置文件? (y/n): "
        read -r REPLY
        case "$REPLY" in
            [Yy]*) info "配置文件将被删除。" ;;
            *) info "将保留配置文件: ${CONFIG_FILE}"; DELETE_CONFIG=0 ;;
        esac
    fi

    info "停止并禁用服务..."
    systemctl stop "${SERVICE_NAME}" >/dev/null 2>&1 || true
    systemctl disable "${SERVICE_NAME}" >/dev/null 2>&1 || true
    
    info "删除 systemd 服务文件..."
    rm -f "$SERVICE_FILE_PATH"
    systemctl daemon-reload

    info "删除安装目录..."
    if [ "$DELETE_CONFIG" -eq 1 ]; then
        rm -rf "$INSTALL_DIR"
    else
        # 只删除程序文件，保留配置文件
        find "$INSTALL_DIR" -mindepth 1 ! -name 'config.json' -exec rm -rf {} +
        # 如果目录只剩下配置文件，则保留目录
    fi
    
    info "${GREEN}卸载完成！${NC}"
}

# 主逻辑
main() {
    check_os
    if [ "$(id -u)" -ne 0 ]; then
        die "此脚本需要以 root 权限运行。请使用 sudo。"
    fi
    
    case "$1" in
        install)
            install_service
            ;;
        uninstall)
            uninstall_service
            ;;
        *)
            printf "用法: %s {install|uninstall}\n" "$0"
            exit 1
            ;;
    esac
}

main "$@"