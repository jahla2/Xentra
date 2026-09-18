from __future__ import annotations

import os
from typing import Any

from app.providers import (
    HeuristicProvider,
    InvestigationProvider,
    LLMConfig,
    OpenAICompatibleProvider,
    ProviderError,
)
from app.schemas import Environment, Evidence, InvestigationFinding, InvestigationRequest


class InvestigationEngine:
    def __init__(
        self,
        provider: InvestigationProvider,
        fallback: InvestigationProvider | None = None,
    ) -> None:
        self.provider = provider
        self.fallback = fallback

    @property
    def provider_name(self) -> str:
        return self.provider.name

    def investigate(self, request: InvestigationRequest) -> InvestigationFinding:
        try:
            return self.provider.investigate(request)
        except ProviderError:
            if self.fallback is None:
                raise
            return self.fallback.investigate(request)


def build_engine_from_env() -> InvestigationEngine:
    api_key = os.getenv("XENTRA_LLM_API_KEY", "").strip()
    model = os.getenv("XENTRA_LLM_MODEL", "").strip()

    if not api_key or not model:
        return InvestigationEngine(provider=HeuristicProvider())

    config = LLMConfig(
        base_url=os.getenv("XENTRA_LLM_BASE_URL", "https://api.openai.com/v1"),
        api_key=api_key,
        model=model,
        timeout_seconds=float(os.getenv("XENTRA_LLM_TIMEOUT_SECONDS", "20")),
        max_evidence_chars=int(os.getenv("XENTRA_LLM_MAX_EVIDENCE_CHARS", "24000")),
        max_item_chars=int(os.getenv("XENTRA_LLM_MAX_ITEM_CHARS", "6000")),
    )
    return InvestigationEngine(
        provider=OpenAICompatibleProvider(config),
        fallback=HeuristicProvider(),
    )


def investigate(
    question: str,
    evidence: list[dict[str, Any]],
    environment: dict[str, Any] | None = None,
) -> dict[str, str]:
    request = InvestigationRequest(
        environment=Environment.model_validate(
            environment
            or {
                "id": "unknown",
                "name": "Unknown environment",
                "type": "unknown",
            }
        ),
        question=question,
        evidence=[Evidence.model_validate(item) for item in evidence],
    )
    return HeuristicProvider().investigate(request).model_dump()
