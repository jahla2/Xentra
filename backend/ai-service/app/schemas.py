from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field


class Evidence(BaseModel):
    source: str
    output: str
    success: bool


class Environment(BaseModel):
    id: str
    name: str
    type: str
    connectionType: str = "runner"
    runnerUrl: str | None = None
    sshHost: str | None = None
    os: str = ""
    hostname: str = ""
    capabilities: list[str] = Field(default_factory=list)


class InvestigationRequest(BaseModel):
    environment: Environment
    question: str
    evidence: list[Evidence]


class InvestigationFinding(BaseModel):
    summary: str
    confidence: Literal["low", "medium", "high"]
    probableRootCause: str
    recommendedAction: str
