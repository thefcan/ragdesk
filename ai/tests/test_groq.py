import httpx
import pytest

from app import chat


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


def test_groq_chat_streams_sse(mock_httpx, monkeypatch):
    sse = (
        'data: {"choices":[{"delta":{"content":"Hello"}}]}\n\n'
        'data: {"choices":[{"delta":{"content":" world"}}]}\n\n'
        "data: [DONE]\n\n"
    )

    def handler(request):
        assert request.url.path.endswith("/chat/completions")
        assert request.headers["authorization"] == "Bearer test-key"
        assert "llama-3.3-70b-versatile" in request.content.decode()
        return httpx.Response(200, text=sse)

    mock_httpx(handler)
    monkeypatch.setattr(chat.settings, "groq_api_key", "test-key")

    tokens = list(chat.GroqChat().stream("the prompt"))
    assert "".join(tokens) == "Hello world"


def test_get_chat_provider_selects_groq(monkeypatch):
    monkeypatch.setattr(chat.settings, "chat_provider", "groq")
    assert isinstance(chat.get_chat_provider(), chat.GroqChat)
