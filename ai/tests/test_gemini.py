import httpx
import pytest

from app import chat, embeddings


@pytest.fixture
def mock_httpx(monkeypatch):
    """Back httpx.Client with a MockTransport so no real API is called."""
    real_client = httpx.Client

    def install(handler):
        def factory(**kwargs):
            kwargs["transport"] = httpx.MockTransport(handler)
            return real_client(**kwargs)

        monkeypatch.setattr(httpx, "Client", factory)

    return install


def test_gemini_embedder(mock_httpx, monkeypatch):
    def handler(request):
        assert "text-embedding-004:batchEmbedContents" in str(request.url)
        assert request.url.params.get("key") == "test-key"
        return httpx.Response(
            200, json={"embeddings": [{"values": [0.1, 0.2]}, {"values": [0.3, 0.4]}]}
        )

    mock_httpx(handler)
    monkeypatch.setattr(embeddings.settings, "gemini_api_key", "test-key")

    out = embeddings.GeminiEmbedder().embed(["a", "b"])
    assert out == [[0.1, 0.2], [0.3, 0.4]]


def test_gemini_chat_streams_sse(mock_httpx, monkeypatch):
    sse = (
        'data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}\n\n'
        'data: {"candidates":[{"content":{"parts":[{"text":" world"}]}}]}\n\n'
    )

    def handler(request):
        assert "gemini-2.0-flash:streamGenerateContent" in str(request.url)
        assert request.url.params.get("alt") == "sse"
        return httpx.Response(200, text=sse)

    mock_httpx(handler)
    monkeypatch.setattr(chat.settings, "gemini_api_key", "test-key")

    tokens = list(chat.GeminiChat().stream("the prompt"))
    assert "".join(tokens) == "Hello world"
