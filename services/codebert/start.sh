#!/bin/bash

# CodeBERT 服务启动脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 检查依赖是否安装
if ! python3 -c "import torch" 2>/dev/null; then
    echo "📥 安装依赖（这可能需要 10-30 分钟）..."
    pip3 install --upgrade pip
    pip3 install -r requirements.txt
fi

# 启动服务
echo "🚀 启动 CodeBERT 服务..."
echo "   服务地址: http://localhost:8001"
echo "   按 Ctrl+C 停止服务"
echo ""

python3 main.py

