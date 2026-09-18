from fastapi import FastAPI

from app.investigator import build_engine_from_env
from app.schemas import (
    AgentDecision,
    AgentInvestigationRequest,
    InvestigationFinding,
    InvestigationRequest,
)


app = FastAPI(title="Xentra AI Service", version="0.4.0")
engine = build_engine_from_env()


@app.get("/health")
def health() -> dict[str, str]:
    return {
        "status": "ok",
        "service": "ai-service",
        "investigator": engine.provider_name,
    }


@app.post("/v1/investigate", response_model=InvestigationFinding)
def run_investigation(request: InvestigationRequest) -> InvestigationFinding:
    return engine.investigate(request)


@app.post("/v1/investigate/next", response_model=AgentDecision)
def next_investigation_step(request: AgentInvestigationRequest) -> AgentDecision:
    return engine.next_step(request)
