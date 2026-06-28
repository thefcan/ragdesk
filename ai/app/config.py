"""Runtime configuration loaded from the environment."""

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    port: int = 8000
    database_url: str = "postgresql://ragdesk:ragdesk@localhost:5432/ragdesk"
    ragdesk_env: str = "development"
    # Shared secret the Go API sends; empty disables the check (dev only).
    internal_token: str = ""

    # Provider-agnostic embeddings. "ollama" calls a local model; "fake" yields
    # deterministic vectors so tests and CI need no model server.
    embedding_provider: str = "ollama"
    ollama_base_url: str = "http://localhost:11434"
    embedding_model: str = "nomic-embed-text"
    embedding_dim: int = 768

    chunk_size: int = 1000
    chunk_overlap: int = 150

    # Provider-agnostic chat. "fake" yields a deterministic answer for tests/CI.
    chat_provider: str = "ollama"
    chat_model: str = "llama3.2:3b"
    retrieval_k: int = 4


settings = Settings()
