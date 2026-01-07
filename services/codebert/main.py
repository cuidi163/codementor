"""
CodeBERT Embedding Service
A FastAPI microservice that provides code embedding using Microsoft's CodeBERT model.
"""

import os
import logging
from typing import List, Optional
from contextlib import asynccontextmanager

import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from transformers import AutoTokenizer, AutoModel

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Global model and tokenizer
model = None
tokenizer = None
device = None


class EmbeddingRequest(BaseModel):
    """Request model for embedding generation"""
    text: str
    max_length: Optional[int] = 512


class BatchEmbeddingRequest(BaseModel):
    """Request model for batch embedding generation"""
    texts: List[str]
    max_length: Optional[int] = 512


class EmbeddingResponse(BaseModel):
    """Response model for embedding"""
    embedding: List[float]
    dimension: int


class BatchEmbeddingResponse(BaseModel):
    """Response model for batch embeddings"""
    embeddings: List[List[float]]
    dimension: int
    count: int


class HealthResponse(BaseModel):
    """Response model for health check"""
    status: str
    model: str
    device: str
    dimension: int


def load_model():
    """Load CodeBERT model and tokenizer"""
    global model, tokenizer, device
    
    model_name = os.getenv("MODEL_NAME", "microsoft/codebert-base")
    logger.info(f"Loading model: {model_name}")
    
    # Determine device
    if torch.cuda.is_available():
        device = torch.device("cuda")
        logger.info("Using CUDA GPU")
    elif torch.backends.mps.is_available():
        device = torch.device("mps")
        logger.info("Using Apple MPS")
    else:
        device = torch.device("cpu")
        logger.info("Using CPU")
    
    # Load tokenizer and model
    tokenizer = AutoTokenizer.from_pretrained(model_name)
    model = AutoModel.from_pretrained(model_name)
    model.to(device)
    model.eval()
    
    logger.info(f"Model loaded successfully on {device}")
    return model, tokenizer


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Lifespan context manager for model loading"""
    load_model()
    yield
    # Cleanup (if needed)
    logger.info("Shutting down...")


# Create FastAPI app
app = FastAPI(
    title="CodeBERT Embedding Service",
    description="Generate code embeddings using Microsoft's CodeBERT model",
    version="1.0.0",
    lifespan=lifespan
)


def generate_embedding(text: str, max_length: int = 512) -> List[float]:
    """Generate embedding for a single text"""
    global model, tokenizer, device
    
    if model is None or tokenizer is None:
        raise RuntimeError("Model not loaded")
    
    # Tokenize input
    inputs = tokenizer(
        text,
        return_tensors="pt",
        padding=True,
        truncation=True,
        max_length=max_length
    )
    
    # Move to device
    inputs = {k: v.to(device) for k, v in inputs.items()}
    
    # Generate embedding
    with torch.no_grad():
        outputs = model(**inputs)
        # Use mean pooling of last hidden state
        embeddings = outputs.last_hidden_state.mean(dim=1)
    
    # Convert to list and return
    return embeddings[0].cpu().tolist()


def generate_batch_embeddings(texts: List[str], max_length: int = 512) -> List[List[float]]:
    """Generate embeddings for multiple texts"""
    global model, tokenizer, device
    
    if model is None or tokenizer is None:
        raise RuntimeError("Model not loaded")
    
    # Tokenize inputs
    inputs = tokenizer(
        texts,
        return_tensors="pt",
        padding=True,
        truncation=True,
        max_length=max_length
    )
    
    # Move to device
    inputs = {k: v.to(device) for k, v in inputs.items()}
    
    # Generate embeddings
    with torch.no_grad():
        outputs = model(**inputs)
        # Use mean pooling of last hidden state
        embeddings = outputs.last_hidden_state.mean(dim=1)
    
    # Convert to list and return
    return embeddings.cpu().tolist()


@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint"""
    if model is None:
        raise HTTPException(status_code=503, detail="Model not loaded")
    
    # Get embedding dimension
    test_embedding = generate_embedding("test")
    
    return HealthResponse(
        status="healthy",
        model="microsoft/codebert-base",
        device=str(device),
        dimension=len(test_embedding)
    )


@app.post("/embed", response_model=EmbeddingResponse)
async def embed_text(request: EmbeddingRequest):
    """Generate embedding for a single text"""
    try:
        embedding = generate_embedding(request.text, request.max_length)
        return EmbeddingResponse(
            embedding=embedding,
            dimension=len(embedding)
        )
    except Exception as e:
        logger.error(f"Error generating embedding: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/embed/batch", response_model=BatchEmbeddingResponse)
async def embed_batch(request: BatchEmbeddingRequest):
    """Generate embeddings for multiple texts"""
    try:
        if len(request.texts) == 0:
            return BatchEmbeddingResponse(
                embeddings=[],
                dimension=768,
                count=0
            )
        
        embeddings = generate_batch_embeddings(request.texts, request.max_length)
        return BatchEmbeddingResponse(
            embeddings=embeddings,
            dimension=len(embeddings[0]) if embeddings else 768,
            count=len(embeddings)
        )
    except Exception as e:
        logger.error(f"Error generating batch embeddings: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/")
async def root():
    """Root endpoint"""
    return {
        "service": "CodeBERT Embedding Service",
        "version": "1.0.0",
        "endpoints": {
            "health": "/health",
            "embed": "/embed",
            "embed_batch": "/embed/batch"
        }
    }


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", 8000))
    uvicorn.run(app, host="0.0.0.0", port=port)

