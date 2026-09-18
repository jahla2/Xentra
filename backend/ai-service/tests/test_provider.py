import json

import httpx
import pytest

from app.providers import LLMConfig, OpenAICompatibleProvider, ProviderError
from app.schemas import Environment, Evidence, InvestigationRequest


def make_request() -> InvestigationRequest:
    return InvestigationRequest(
        environment=Environment(
            id="env-1",
            name="Production",
            type="production",
            connectionType="ssh",
            hostname="prod-01",
            os="linux",
            capabilities=["docker", "systemd"],
        ),
        question="Why did the deployment fail?",
        evidence=[
            Evidence(
                source="docker.logs:api",
                success=True,
                output="connection refused to postgres",
            )
        ],
    )


def test_openai_compatible_provider_validates_structured_result() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/v1/chat/completions"
        assert request.headers["authorization"] == "Bearer test-key"
        body = json.loads(request.content)
        assert body["model"] == "test-model"
        assert "docker.logs:api" in body["messages"][1]["content"]
        return httpx.Response(
            200,
            json={
                "choices": [
                    {
                        "message": {
                            "content": json.dumps(
                                {
                                    "summary": "Database connectivity failed after deployment.",
                                    "confidence": "high",
                                    "probableRootCause": "The API cannot reach PostgreSQL.",
                                    "recommendedAction": "Verify the database host and port before approving remediation.",
                                }
                            )
                        }
                    }
                ]
            },
        )

    client = httpx.Client(transport=httpx.MockTransport(handler))
    provider = OpenAICompatibleProvider(
        LLMConfig(
            base_url="https://llm.example/v1",
            api_key="test-key",
            model="test-model",
        ),
        client=client,
    )

    result = provider.investigate(make_request())

    assert result.confidence == "high"
    assert "PostgreSQL" in result.probableRootCause


def test_provider_rejects_invalid_confidence() -> None:
    def handler(_: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,
            json={
                "choices": [
                    {
                        "message": {
                            "content": json.dumps(
                                {
                                    "summary": "Bad response",
                                    "confidence": "certain",
                                    "probableRootCause": "Unknown",
                                    "recommendedAction": "Unknown",
                                }
                            )
                        }
                    }
                ]
            },
        )

    provider = OpenAICompatibleProvider(
        LLMConfig(
            base_url="https://llm.example/v1",
            api_key="test-key",
            model="test-model",
        ),
        client=httpx.Client(transport=httpx.MockTransport(handler)),
    )

    with pytest.raises(ProviderError):
        provider.investigate(make_request())


def test_provider_enforces_evidence_size_budget() -> None:
    captured = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["body"] = json.loads(request.content)
        return httpx.Response(
            200,
            json={
                "choices": [
                    {
                        "message": {
                            "content": json.dumps(
                                {
                                    "summary": "Insufficient evidence.",
                                    "confidence": "low",
                                    "probableRootCause": "Unknown.",
                                    "recommendedAction": "Collect more logs.",
                                }
                            )
                        }
                    }
                ]
            },
        )

    request = make_request()
    request.evidence[0].output = "x" * 5000
    provider = OpenAICompatibleProvider(
        LLMConfig(
            base_url="https://llm.example/v1",
            api_key="test-key",
            model="test-model",
            max_evidence_chars=100,
            max_item_chars=80,
        ),
        client=httpx.Client(transport=httpx.MockTransport(handler)),
    )

    provider.investigate(request)

    prompt = captured["body"]["messages"][1]["content"]
    assert "x" * 80 in prompt
    assert "x" * 81 not in prompt
