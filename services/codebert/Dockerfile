# CodeBERT Embedding Service Dockerfile
FROM python:3.11-slim

WORKDIR /app

# Install system dependencies (including ca-certificates for SSL)
RUN apt-get update && apt-get install -y \
    gcc \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 设置所有 SSL 禁用环境变量（构建时和运行时都需要）
ENV PYTHONHTTPSVERIFY=0
ENV CURL_CA_BUNDLE=""
ENV REQUESTS_CA_BUNDLE=""
ENV SSL_CERT_FILE=""
ENV HF_HUB_DISABLE_SSL_VERIFY=1
ENV HF_HUB_DISABLE_XET=1
ENV PORT=8001
ENV MODEL_NAME=microsoft/codebert-base

# Copy requirements first for better caching
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir --trusted-host pypi.org --trusted-host files.pythonhosted.org -r requirements.txt

# Copy download script
COPY download_model.py .

# 在构建时下载模型（使用我们创建的脚本）
RUN python3 download_model.py

# Copy application code
COPY main.py .

# Expose port
EXPOSE 8001

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8001/health')" || exit 1

# Run the service
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8001"]

