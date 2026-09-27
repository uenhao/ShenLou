#!/bin/bash
# ShenLou 一键构建脚本
# 前置条件（只需一次）：
#   sudo apt install -y libgtk-3-dev libwebkit2gtk-4.1-dev
# 之后运行：./build.sh

set -e
export PATH="$HOME/.local/bin:$HOME/GoPath/bin:$PATH"
export GOPATH="$HOME/GoPath"
export GOBIN="$HOME/GoPath/bin"

cd "$(dirname "$0")"

# Ubuntu 24.04 / Mint 22 的 webkit 是 4.1 API，需要 webkit2_41 标签
TAGS="webkit2_41"

wails build -tags "$TAGS" -ldflags "-X main.buildID=$(date +%Y%m%d%H%M%S)"

echo
echo "构建完成：$(pwd)/build/bin/ShenLou"
rm -f "$(pwd)/build/bin/radiohub"  # 清理旧名产物
