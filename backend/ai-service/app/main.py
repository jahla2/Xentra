from fastapi import FastAPI
from pydantic import BaseModel

from app.investigator import investigate

app = FastAPI(title="Xentra AI Service", version="0.2.0")

class Evidence(BaseModel):
    source: str
    output: str
    success: bool

class Environment(BaseModel):
    id: str
    name: str
    type: str
    runnerUrl: str
    os: str = ""
    hostname: str = ""
    capabilities: list[str] = []

class InvestigationRequest(BaseModel):
    environment: Environment
    question: str
    evidence: list[Evidence]

@app.get("/health")
def health() -> dict[str, str]:
    return {"status":"ok","service":"ai-service"}

@app.post("/v1/investigate")
def run_investigation(request: InvestigationRequest) -> dict[str, str]:
    return investigate(request.question, [item.model_dump() for item in request.evidence])
