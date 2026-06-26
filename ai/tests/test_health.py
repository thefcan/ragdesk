"""Smoke tests for the probe endpoints (no database required)."""

from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)


def test_healthz():
    resp = client.get("/healthz")
    assert resp.status_code == 200
    assert resp.json() == {"status": "ok", "service": "ragdesk-ai"}


def test_version():
    resp = client.get("/version")
    assert resp.status_code == 200
    body = resp.json()
    assert body["service"] == "ragdesk-ai"
    assert "version" in body
