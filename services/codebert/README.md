# CodeBERT Embedding Service

CodeBERT 微服务，提供代码 embedding 功能。

## 依赖说明

安装 `requirements.txt` 会安装：
- **包数量**：~100-150 个 Python 包
- **磁盘空间**：~2.5-4GB (CPU版本) 或 ~5-7GB (CUDA版本)
- **主要包**：
  - `torch>=2.0.0` - PyTorch 深度学习框架 (~2GB)
  - `transformers==4.37.0` - HuggingFace Transformers (~500MB)
  - `fastapi==0.109.0` - Web 框架 (~5MB)
  - `uvicorn[standard]==0.27.0` - ASGI 服务器 (~2MB)
  - `pydantic==2.5.3` - 数据验证 (~10MB)
- **安装时间**：10-30 分钟（取决于网络速度）

## 本地运行（推荐，避免 Docker SSL 问题）

如果你的网络环境有 TLS 拦截，建议本地运行：

```bash
# 1. 安装依赖
cd services/codebert
pip3 install --upgrade pip
pip3 install -r requirements.txt

# 2. 启动服务（会自动下载模型，首次需要一些时间）
python3 main.py

# 服务会在 http://localhost:8001 启动

# 或者使用启动脚本（自动检查并安装依赖）
./start.sh
```

## Docker 运行

如果网络环境正常，可以使用 Docker：

```bash
# 构建镜像
docker build -t codebert-service .

# 运行容器
docker run -d --name codebert_service -p 8001:8001 codebert-service
```

## 测试服务

```bash
# 健康检查
curl http://localhost:8001/health

# 生成 embedding
curl -X POST http://localhost:8001/embed \
  -H "Content-Type: application/json" \
  -d '{"text": "def hello(): pass"}'
```

## 注意事项

- 首次运行需要下载 CodeBERT 模型（约 500MB），需要访问 HuggingFace
- 如果网络有 TLS 拦截，代码已自动禁用 SSL 验证
- 模型会缓存在 `~/.cache/huggingface/` 目录

