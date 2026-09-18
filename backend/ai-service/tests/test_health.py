from fastapi.testclient import TestClient

from app.main import app


def test_health_returns_ai_service_status() -> None:
    response = TestClient(app).get("/health")

    assert response.status_code == 200
    payload = response.json()
    assert payload["status"] == "ok"
    assert payload["service"] == "ai-service"
    assert payload["investigator"] in {"heuristic", "openai-compatible"}
