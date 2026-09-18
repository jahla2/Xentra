from __future__ import annotations

from datetime import datetime
from typing import Literal

from pydantic import BaseModel, Field, model_validator


class Evidence(BaseModel):
    source: str
    output: str
    success: bool
    occurredAt: datetime | None = None
    durationMs: int = Field(default=0, ge=0)


class Environment(BaseModel):
    id: str
    projectId: str | None = None
    name: str
    type: str
    connectionType: str = "runner"
    runnerUrl: str | None = None
    sshHost: str | None = None
    os: str = ""
    hostname: str = ""
    capabilities: list[str] = Field(default_factory=list)


class ToolRequest(BaseModel):
    tool: str
    arguments: dict[str, str] = Field(default_factory=dict)


class InvestigationRequest(BaseModel):
    environment: Environment
    question: str
    evidence: list[Evidence]


class AgentInvestigationRequest(InvestigationRequest):
    availableTools: list[str] = Field(default_factory=list)
    remainingSteps: int = Field(default=1, ge=1, le=3)


class InvestigationFinding(BaseModel):
    summary: str
    confidence: Literal["low", "medium", "high"]
    probableRootCause: str
    recommendedAction: str


class AgentDecision(BaseModel):
    mode: Literal["complete", "tools"]
    toolRequests: list[ToolRequest] = Field(default_factory=list, max_length=3)
    finding: InvestigationFinding | None = None

    @model_validator(mode="after")
    def validate_mode_payload(self) -> "AgentDecision":
        if self.mode == "complete" and self.finding is None:
            raise ValueError("complete decisions require a finding")
        if self.mode == "tools" and not self.toolRequests:
            raise ValueError("tools decisions require at least one tool request")
        return self


class EmbeddingRequest(BaseModel):
    texts: list[str] = Field(min_length=1, max_length=32)


class EmbeddingResponse(BaseModel):
    vectors: list[list[float]]
    dimensions: int
    provider: str
