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


def test_build_prompt_includes_conversation_history():
    sources = [{"document_id": "d1", "title": "Handbook", "content": "Free plan: 25 docs."}]
    history = [
        {"role": "user", "content": "How many docs on Free?"},
        {"role": "assistant", "content": "The Free plan allows 25 documents."},
    ]
    prompt = build_prompt(sources, "And on Pro?", history)
    assert "Conversation so far:" in prompt
    assert "User: How many docs on Free?" in prompt
    assert "Assistant: The Free plan allows 25 documents." in prompt
    # The current question still comes last.
    assert prompt.rstrip().endswith("Question: And on Pro?\nAnswer:")


def test_build_prompt_omits_history_section_when_empty():
    prompt = build_prompt([], "hi", [])
    assert "Conversation so far:" not in prompt


def test_build_prompt_caps_history_turns():
    long_history = [{"role": "user", "content": f"msg{i}"} for i in range(20)]
    prompt = build_prompt([], "now?", long_history)
    # Only the most recent MAX_HISTORY_TURNS are folded in.
    assert "msg19" in prompt
    assert "msg0" not in prompt


def test_fake_chat_streams_tokens():
    tokens = list(FakeChat().stream("prompt"))
    assert len(tokens) > 1
    assert "".join(tokens).strip().endswith("answer.")
