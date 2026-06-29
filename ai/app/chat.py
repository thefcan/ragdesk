"""Provider-agnostic chat: a streaming Ollama client and a deterministic fake."""

import json
from abc import ABC, abstractmethod
from collections.abc import Iterator

import httpx

from app.config import settings


class ChatProvider(ABC):
    @abstractmethod
    def stream(self, prompt: str) -> Iterator[str]: ...


class OllamaChat(ChatProvider):
    """Streams tokens from a local Ollama generation model (free, $0)."""

    def stream(self, prompt: str) -> Iterator[str]:
        with httpx.Client(base_url=settings.ollama_base_url, timeout=120.0) as client:
            with client.stream(
                "POST",
                "/api/generate",
                json={"model": settings.chat_model, "prompt": prompt, "stream": True},
            ) as resp:
                resp.raise_for_status()
                for line in resp.iter_lines():
                    if not line:
                        continue
                    obj = json.loads(line)
                    token = obj.get("response")
                    if token:
                        yield token
                    if obj.get("done"):
                        break


class GeminiChat(ChatProvider):
    """Streams tokens from Google's Gemini chat API (free tier) over SSE."""

    def stream(self, prompt: str) -> Iterator[str]:
        model = settings.gemini_chat_model
        payload = {"contents": [{"parts": [{"text": prompt}]}]}
        with httpx.Client(base_url=settings.gemini_base_url, timeout=120.0) as client:
            with client.stream(
                "POST",
                f"/models/{model}:streamGenerateContent",
                params={"key": settings.gemini_api_key, "alt": "sse"},
                json=payload,
            ) as resp:
                resp.raise_for_status()
                for line in resp.iter_lines():
                    if not line.startswith("data:"):
                        continue
                    obj = json.loads(line[len("data:") :].strip())
                    for candidate in obj.get("candidates", []):
                        for part in candidate.get("content", {}).get("parts", []):
                            token = part.get("text")
                            if token:
                                yield token


class FakeChat(ChatProvider):
    """Deterministic answer so tests/CI need no model server."""

    def stream(self, prompt: str) -> Iterator[str]:
        yield from ["Based ", "on ", "the ", "context, ", "here ", "is ", "the ", "answer."]


def get_chat_provider() -> ChatProvider:
    if settings.chat_provider == "fake":
        return FakeChat()
    if settings.chat_provider == "gemini":
        return GeminiChat()
    return OllamaChat()


def build_prompt(sources: list[dict], question: str) -> str:
    if not sources:
        context = "(no relevant documents found)"
    else:
        context = "\n\n".join(
            f"[{i + 1}] {s['title']}:\n{s['content']}" for i, s in enumerate(sources)
        )
    return (
        "You are a helpful assistant. Answer the question using ONLY the context below. "
        "If the answer is not in the context, say you don't know. Cite sources by their number.\n\n"
        f"Context:\n{context}\n\nQuestion: {question}\nAnswer:"
    )
