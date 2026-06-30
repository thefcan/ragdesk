"""Runtime configuration loaded from the environment."""

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    port: int = 8000
    database_url: str = "postgresql://ragdesk:ragdesk@localhost:5432/ragdesk"
    ragdesk_env: str = "development"
    # Shared secret the Go API sends; empty disables the check (dev only).
    internal_token: str = ""

    # Provider-agnostic embeddings. "ollama" calls a local model; "gemini" calls
    # Google's free-tier API; "fake" yields deterministic vectors so tests and CI
    # need no model server.
    embedding_provider: str = "ollama"
    ollama_base_url: str = "http://localhost:11434"
    embedding_model: str = "nomic-embed-text"
    embedding_dim: int = 768

    # Google Gemini (free tier). One key serves both chat and embeddings.
    # gemini-embedding-001 is requested at 768 dims (outputDimensionality) to
    # match the schema, so no migration.
    gemini_api_key: str = ""
    gemini_base_url: str = "https://generativelanguage.googleapis.com/v1beta"
    gemini_embedding_model: str = "gemini-embedding-001"
    gemini_chat_model: str = "gemini-2.5-flash"

    # Groq (free tier). OpenAI-compatible hosted chat — chat only, so pair it
    # with any embeddings provider above. Key at https://console.groq.com.
    groq_api_key: str = ""
    groq_base_url: str = "https://api.groq.com/openai/v1"
    groq_chat_model: str = "llama-3.3-70b-versatile"

    chunk_size: int = 1000
    chunk_overlap: int = 150

    # Provider-agnostic chat. "fake" yields a deterministic answer for tests/CI.
    chat_provider: str = "ollama"
    chat_model: str = "llama3.2:3b"
    retrieval_k: int = 4
    # Max cosine distance for a chunk to count as relevant (0 = identical).
    retrieval_max_distance: float = 0.8


settings = Settings()
