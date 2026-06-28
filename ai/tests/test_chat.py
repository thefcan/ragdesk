"""Unit tests for prompt building and the fake chat provider (no deps)."""

from app.chat import FakeChat, build_prompt


def test_build_prompt_includes_context_and_question():
    sources = [{"document_id": "d1", "title": "Handbook", "content": "Remote-first policy."}]
    prompt = build_prompt(sources, "What is the policy?")
    assert "Handbook" in prompt
    assert "Remote-first policy." in prompt
    assert "What is the policy?" in prompt


def test_build_prompt_handles_no_sources():
    prompt = build_prompt([], "anything?")
    assert "no relevant documents" in prompt


def test_fake_chat_streams_tokens():
    tokens = list(FakeChat().stream("prompt"))
    assert len(tokens) > 1
    assert "".join(tokens).strip().endswith("answer.")
