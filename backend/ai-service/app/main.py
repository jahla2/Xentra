from fastapi import FastAPI

from app.investigator import build_engine_from_env
from app.schemas import InvestigationFinding, InvestigationRequest


app = FastAPI(title="Xentra AI Service", version="0.3.0")
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
