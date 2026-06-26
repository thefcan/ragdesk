"""Runtime configuration loaded from the environment."""

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    port: int = 8000
    database_url: str = "postgresql://ragdesk:ragdesk@localhost:5432/ragdesk"
    # Provider-agnostic LLM: defaults to a free, local Ollama instance.
    ollama_base_url: str = "http://localhost:11434"
    ragdesk_env: str = "development"


settings = Settings()
