"""Unit tests for the text chunker (no external dependencies)."""

from app.chunking import chunk_text


def test_empty_text():
    assert chunk_text("") == []
    assert chunk_text("   ") == []


def test_short_text_is_one_chunk():
    assert chunk_text("hello world", size=1000) == ["hello world"]


def test_long_text_splits_with_overlap():
    text = "abcdefghij" * 250  # 2500 chars
    chunks = chunk_text(text, size=1000, overlap=200)
    assert len(chunks) >= 3
    assert all(len(c) <= 1000 for c in chunks)
    # consecutive chunks overlap
    assert chunks[0][-200:] == chunks[1][:200]
