from fastapi import FastAPI

from app.telemetry import setup_tracing


def test_tracing_disabled_is_noop(monkeypatch):
    monkeypatch.delenv("OTEL_EXPORTER_OTLP_ENDPOINT", raising=False)
    app = FastAPI()
    assert setup_tracing(app) is False
