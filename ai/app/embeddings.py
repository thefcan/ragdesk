"""Provider-agnostic embeddings: a real Ollama client and a deterministic fake."""

import hashlib
from abc import ABC, abstractmethod

import httpx

from app.config import settings


class Embedder(ABC):
    @abstractmethod
    def embed(self, texts: list[str]) -> list[list[float]]: ...


class OllamaEmbedder(Embedder):
    """Calls a local Ollama model (free, $0) for embeddings."""

    def embed(self, texts: list[str]) -> list[list[float]]:
        out: list[list[float]] = []
        with httpx.Client(base_url=settings.ollama_base_url, timeout=60.0) as client:
            for text in texts:
                resp = client.post(
                    "/api/embeddings",
                    json={"model": settings.embedding_model, "prompt": text},
                )
                resp.raise_for_status()
                out.append(resp.json()["embedding"])
        return out


class FakeEmbedder(Embedder):
    """Deterministic pseudo-embeddings so tests/CI need no model server."""

    def embed(self, texts: list[str]) -> list[list[float]]:
        dim = settings.embedding_dim
        out: list[list[float]] = []
        for text in texts:
            digest = hashlib.sha256(text.encode("utf-8")).digest()
            out.append([digest[i % len(digest)] / 255.0 for i in range(dim)])
        return out


def get_embedder() -> Embedder:
    if settings.embedding_provider == "fake":
        return FakeEmbedder()
    return OllamaEmbedder()
