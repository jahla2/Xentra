from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Protocol

import httpx
from pydantic import ValidationError

from app.schemas import (
    AgentDecision,
    AgentInvestigationRequest,
    InvestigationFinding,
    InvestigationRequest,
)


class ProviderError(RuntimeError):
    pass


class InvestigationProvider(Protocol):
    name: str

    def investigate(self, request: InvestigationRequest) -> InvestigationFinding:
        ...

    def decide(self, request: AgentInvestigationRequest) -> AgentDecision:
        ...


@dataclass(frozen=True)
class LLMConfig:
    base_url: str
    api_key: str
    model: str
    timeout_seconds: float = 20.0
    max_evidence_chars: int = 24_000
    max_item_chars: int = 6_000


class HeuristicProvider:
    name = "heuristic"

    def investigate(self, request: InvestigationRequest) -> InvestigationFinding:
        combined = "\n".join(item.output for item in request.evidence).lower()

        if "restarting" in combined or "unhealthy" in combined or "exited" in combined:
            return InvestigationFinding(
                summary="Xentra found a container health signal that likely explains the incident.",
                confidence="high",
                probableRootCause="One or more application containers are unhealthy or repeatedly restarting.",
                recommendedAction="Inspect the affected container logs and configuration before approving a restart.",
            )

        if "connection refused" in combined or "could not connect" in combined:
            return InvestigationFinding(
                summary="Xentra found a downstream connectivity failure.",
                confidence="high",
                probableRootCause="The application cannot connect to a required downstream service.",
                recommendedAction="Verify the target service, hostname, port, and current deployment configuration.",
            )

        return InvestigationFinding(
            summary="No strong failure signature was found in the collected evidence.",
            confidence="low",
            probableRootCause="Insufficient evidence to identify a probable root cause.",
            recommendedAction="Collect application-specific logs or add a health endpoint to narrow the investigation.",
        )

    def decide(self, request: AgentInvestigationRequest) -> AgentDecision:
        return AgentDecision(mode="complete", finding=self.investigate(request))


class OpenAICompatibleProvider:
    name = "openai-compatible"

    def __init__(self, config: LLMConfig, client: httpx.Client | None = None) -> None:
        self._config = config
        self._client = client or httpx.Client(timeout=config.timeout_seconds)

    def investigate(self, request: InvestigationRequest) -> InvestigationFinding:
        system = (
            "You are Xentra, an evidence-grounded DevOps incident investigator. "
            "Use only supplied evidence. Never invent logs, deployment events, commands, credentials, "
            "or infrastructure state. If evidence is insufficient, lower confidence. Recommended actions "
            "are advisory and cannot bypass human approval. Return JSON only with exactly: summary, "
            "confidence, probableRootCause, recommendedAction. confidence must be low, medium, or high."
        )
        parsed = self._call_json(system, self._build_prompt(request))
        try:
            return InvestigationFinding.model_validate(parsed)
        except ValidationError as exc:
            raise ProviderError(f"LLM provider returned an invalid investigation response: {exc}") from exc

    def decide(self, request: AgentInvestigationRequest) -> AgentDecision:
        tool_catalog = "\n".join(
            f"- {tool}: {TOOL_DESCRIPTIONS.get(tool, 'read-only diagnostic tool')}"
            for tool in request.availableTools
        )
        system = (
            "You are Xentra's bounded DevOps investigation agent. Use only supplied evidence and the "
            "listed read-only typed tools. Never request mutation, shell, file-write, restart, delete, "
            "credential, or arbitrary-command tools. If you have enough evidence, return "
            '{"mode":"complete","toolRequests":[],"finding":{"summary":"...","confidence":"low|medium|high",'
            '"probableRootCause":"...","recommendedAction":"..."}}. '
            "If more evidence is necessary, return "
            '{"mode":"tools","toolRequests":[{"tool":"...","arguments":{"key":"value"}}],"finding":null}. '
            "Request at most three tools and only tools from the supplied list."
        )
        user = (
            self._build_prompt(request)
            + f"\n\nRemaining investigation steps: {request.remainingSteps}"
            + "\nAvailable read-only tools:\n"
            + tool_catalog
        )
        parsed = self._call_json(system, user)
        try:
            return AgentDecision.model_validate(parsed)
        except ValidationError as exc:
            raise ProviderError(f"LLM provider returned an invalid agent decision: {exc}") from exc

    def _call_json(self, system: str, user: str) -> dict:
        payload = {
            "model": self._config.model,
            "temperature": 0.1,
            "response_format": {"type": "json_object"},
            "messages": [
                {"role": "system", "content": system},
                {"role": "user", "content": user},
            ],
        }
        try:
            response = self._client.post(
                self._chat_completions_url(),
                headers={
                    "Authorization": f"Bearer {self._config.api_key}",
                    "Content-Type": "application/json",
                },
                json=payload,
            )
            response.raise_for_status()
            body = response.json()
            content = body["choices"][0]["message"]["content"]
            return json.loads(self._strip_json_fence(content))
        except (httpx.HTTPError, KeyError, IndexError, TypeError, json.JSONDecodeError) as exc:
            raise ProviderError(f"LLM provider returned invalid JSON: {exc}") from exc

    def _chat_completions_url(self) -> str:
        return self._config.base_url.rstrip("/") + "/chat/completions"

    def _build_prompt(self, request: InvestigationRequest) -> str:
        remaining = self._config.max_evidence_chars
        evidence_lines: list[str] = []

        for index, item in enumerate(request.evidence, start=1):
            if remaining <= 0:
                break
            output = item.output[: self._config.max_item_chars]
            output = output[:remaining]
            remaining -= len(output)
            evidence_lines.append(
                f"[evidence-{index}] source={item.source} success={item.success}\n{output}"
            )

        environment = request.environment
        capabilities = ", ".join(environment.capabilities) or "unknown"
        return (
            f"Environment: {environment.name} ({environment.type})\n"
            f"Connection: {environment.connectionType}\n"
            f"Host: {environment.hostname or environment.sshHost or 'unknown'}\n"
            f"OS: {environment.os or 'unknown'}\n"
            f"Capabilities: {capabilities}\n"
            f"Question: {request.question}\n\n"
            "Evidence:\n"
            + "\n\n".join(evidence_lines)
        )

    @staticmethod
    def _strip_json_fence(content: str) -> str:
        text = content.strip()
        fence = chr(96) * 3
        if text.startswith(fence):
            lines = text.splitlines()
            if lines:
                lines = lines[1:]
            if lines and lines[-1].strip() == fence:
                lines = lines[:-1]
            text = "\n".join(lines)
        return text.strip()


TOOL_DESCRIPTIONS = {
    "system.info": "kernel and operating system information",
    "system.disk": "filesystem usage",
    "system.cpu": "CPU topology and capabilities",
    "system.memory": "memory usage",
    "system.service_status": "systemd service active state; argument: service",
    "system.journal": "last 200 systemd journal lines; argument: service",
    "docker.list": "running Docker container names and status",
    "docker.logs": "last 200 container log lines; argument: container",
    "docker.inspect": "Docker container metadata; argument: container",
    "docker.stats": "one-shot CPU/memory/network/block metrics; argument: container",
}
