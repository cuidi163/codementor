#!/usr/bin/env python3
"""
在构建时下载 CodeBERT 模型的脚本
"""
import os
import ssl
import sys

# 必须在所有导入之前禁用 SSL
os.environ["PYTHONHTTPSVERIFY"] = "0"
os.environ["CURL_CA_BUNDLE"] = ""
os.environ["REQUESTS_CA_BUNDLE"] = ""
os.environ["SSL_CERT_FILE"] = ""
os.environ["HF_HUB_DISABLE_SSL_VERIFY"] = "1"
os.environ["HF_HUB_DISABLE_EXPERIMENTAL_WARNING"] = "1"
os.environ["TRANSFORMERS_OFFLINE"] = "0"  # 确保在线下载
os.environ["HF_HUB_DISABLE_XET"] = "1"  # 禁用 xet 下载方式，使用传统 HTTP

# 修改默认 SSL context
ssl._create_default_https_context = ssl._create_unverified_context

# Patch urllib3
import urllib3
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

# Patch requests
import requests
from requests.adapters import HTTPAdapter
from urllib3.util.ssl_ import create_urllib3_context

class NoSSLAdapter(HTTPAdapter):
    def init_poolmanager(self, *args, **kwargs):
        ctx = create_urllib3_context()
        ctx.check_hostname = False
        ctx.verify_mode = ssl.CERT_NONE
        kwargs['ssl_context'] = ctx
        return super().init_poolmanager(*args, **kwargs)

# Patch requests.Session
original_init = requests.Session.__init__
def patched_init(self, *args, **kwargs):
    original_init(self, *args, **kwargs)
    self.mount('https://', NoSSLAdapter())
    self.verify = False

requests.Session.__init__ = patched_init

# Patch huggingface_hub 的 HTTP 客户端
try:
    from huggingface_hub.utils import _http
    original_http_backoff = _http.http_backoff
    def patched_http_backoff(*args, **kwargs):
        kwargs.setdefault('verify', False)
        return original_http_backoff(*args, **kwargs)
    _http.http_backoff = patched_http_backoff
except ImportError:
    pass  # huggingface_hub 可能还没导入

# 使用 huggingface_hub 直接下载文件，不需要加载模型
from huggingface_hub import snapshot_download

def download_model():
    """下载 CodeBERT 模型和 tokenizer 的所有文件"""
    model_name = os.getenv("MODEL_NAME", "microsoft/codebert-base")
    print(f"正在下载模型: {model_name}")
    
    try:
        print("下载模型文件（包括 tokenizer 和 model weights）...")
        # snapshot_download 会下载所有必需的文件，包括：
        # - tokenizer 文件
        # - model weights (pytorch_model.bin 或 model.safetensors)
        # - config.json
        # - 其他必需文件
        cache_dir = snapshot_download(
            repo_id=model_name,
            local_files_only=False,
            resume_download=True,
            ignore_patterns=["*.md", "*.txt"],  # 忽略文档文件，只下载必需文件
            local_dir=None,  # 使用默认缓存目录
            local_dir_use_symlinks=False  # 不使用符号链接
        )
        
        print(f"✓ 模型文件下载成功！")
        print(f"  缓存目录: {cache_dir}")
        
        # 验证关键文件是否存在
        required_files = [
            "config.json",
            "tokenizer_config.json"
            # vocab.txt 可能不存在（CodeBERT 使用其他 tokenizer 文件）
        ]
        
        missing_files = []
        for file in required_files:
            file_path = os.path.join(cache_dir, file)
            if not os.path.exists(file_path):
                missing_files.append(file)
        
        if missing_files:
            print(f"⚠️  警告：以下文件未找到: {missing_files}", file=sys.stderr)
        else:
            print("✓ 关键文件验证通过")
        
        # 检查模型权重文件
        model_files = [
            "pytorch_model.bin",
            "model.safetensors",
            "tf_model.h5"
        ]
        found_model_file = False
        for model_file in model_files:
            if os.path.exists(os.path.join(cache_dir, model_file)):
                print(f"✓ 找到模型权重文件: {model_file}")
                found_model_file = True
                break
        
        if not found_model_file:
            print("⚠️  警告：未找到模型权重文件（pytorch_model.bin 或 model.safetensors）", file=sys.stderr)
            print("  这可能是正常的，如果模型使用其他格式", file=sys.stderr)
        
        print(f"✓ 模型 {model_name} 下载完成！")
        return True
    except Exception as e:
        import traceback
        print(f"✗ 下载失败: {e}", file=sys.stderr)
        print(f"详细错误信息:", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    download_model()

