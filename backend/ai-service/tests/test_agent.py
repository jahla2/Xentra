import json

import httpx

from app.investigator import InvestigationEngine
from app.providers import HeuristicProvider, LLMConfig, OpenAICompatibleProvider
from app.schemas import AgentInvestigationRequest, Environment, Evidence


def request() -> AgentInvestigationRequest:
    return AgentInvestigationRequest(
        environment=Environment(
            id="env-1",
            name="Production",
            type="production",
            capabilities=["docker", "systemd"],
        ),
        question="Why is the API down?",
        evidence=[Evidence(source="docker.list", output="api Up", success=True)],
        availableTools=["docker.logs", "docker.inspect", "system.journal"],
        remainingSteps=3,
    )


def test_heuristic_agent_completes_without_tools() -> None:
    engine = InvestigationEngine(HeuristicProvider())
    decision = engine.next_step(request())
    assert decision.mode == "complete"
    assert decision.finding is not None


def test_openai_compatible_agent_can_request_typed_tools() -> None:
    def handler(http_request: httpx.Request) -> httpx.Response:
        payload = json.loads(http_request.content)
        assert "docker.logs" in payload["messages"][1]["content"]
        return httpx.Response(
            200,
            json={
                "choices": [
                    {
                        "message": {
                            "content": json.dumps(
                                {
                                    "mode": "tools",
                                    "toolRequests": [
                                        {"tool": "docker.logs", "arguments": {"container": "api"}}
                                    ],
                                    "finding": None,
                                }
                            )
                        }
                    }
                ]
            },
        )

    provider = OpenAICompatibleProvider(
        LLMConfig(base_url="https://llm.example/v1", api_key="key", model="model"),
        client=httpx.Client(transport=httpx.MockTransport(handler)),
    )
    decision = provider.decide(request())
    assert decision.mode == "tools"
    assert decision.toolRequests[0].tool == "docker.logs"
    assert decision.toolRequests[0].arguments["container"] == "api"
