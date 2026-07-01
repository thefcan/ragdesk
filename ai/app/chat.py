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


class GroqChat(ChatProvider):
    """Streams tokens from Groq's OpenAI-compatible chat API (free tier)."""

    def stream(self, prompt: str) -> Iterator[str]:
        payload = {
            "model": settings.groq_chat_model,
            "messages": [{"role": "user", "content": prompt}],
            "stream": True,
        }
        headers = {"Authorization": f"Bearer {settings.groq_api_key}"}
        with httpx.Client(base_url=settings.groq_base_url, timeout=120.0) as client:
            with client.stream("POST", "/chat/completions", headers=headers, json=payload) as resp:
                resp.raise_for_status()
                for line in resp.iter_lines():
                    if not line.startswith("data:"):
                        continue
                    data = line[len("data:") :].strip()
                    if data == "[DONE]":
                        break
                    obj = json.loads(data)
                    for choice in obj.get("choices", []):
                        token = choice.get("delta", {}).get("content")
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
    if settings.chat_provider == "groq":
        return GroqChat()
    return OllamaChat()


# Cap how much prior conversation is folded into the prompt, so follow-up
# context can never blow up prompt size or cost (the API layer also bounds it).
MAX_HISTORY_TURNS = 8


def build_prompt(sources: list[dict], question: str, history: list[dict] | None = None) -> str:
    if not sources:
        context = "(no relevant documents found)"
    else:
        context = "\n\n".join(
            f"[{i + 1}] {s['title']}:\n{s['content']}" for i, s in enumerate(sources)
        )

    conversation = ""
    if history:
        turns = []
        for turn in history[-MAX_HISTORY_TURNS:]:
            speaker = "User" if turn.get("role") == "user" else "Assistant"
            turns.append(f"{speaker}: {turn.get('content', '')}")
        conversation = "Conversation so far:\n" + "\n".join(turns) + "\n\n"

    return (
        "You are ragdesk, an assistant that answers questions about the user's own documents. "
        "Use the context below to answer as fully and helpfully as you can: you may summarise, "
        "analyse, compare, evaluate and draw reasonable conclusions from it — for example "
        "reviewing or scoring a document the user asks about. Ground what you say in the context "
        "and cite the sources you use by their bracketed number, like [1]. The user may ask "
        "follow-up questions that refer to earlier messages; use the conversation so far to tell "
        "what they mean, but always ground factual claims in the numbered context above. Only if "
        "the context is clearly unrelated to the question should you say you don't have enough "
        "information, and then briefly say what is missing.\n\n"
        f"Context:\n{context}\n\n{conversation}Question: {question}\nAnswer:"
    )
