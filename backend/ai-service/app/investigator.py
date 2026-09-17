from __future__ import annotations

from typing import Any


def investigate(question: str, evidence: list[dict[str, Any]]) -> dict[str, str]:
    combined = "\n".join(str(item.get("output", "")) for item in evidence).lower()
    if "restarting" in combined or "unhealthy" in combined or "exited" in combined:
        return {"summary":"Xentra found a container health signal that likely explains the incident.","confidence":"high","probableRootCause":"One or more application containers are unhealthy or repeatedly restarting.","recommendedAction":"Inspect the affected container logs and configuration before approving a restart."}
    if "connection refused" in combined or "could not connect" in combined:
        return {"summary":"Xentra found a downstream connectivity failure.","confidence":"high","probableRootCause":"The application cannot connect to a required downstream service.","recommendedAction":"Verify the target service, hostname, port, and current deployment configuration."}
    return {"summary":"No strong failure signature was found in the collected evidence.","confidence":"low","probableRootCause":"Insufficient evidence to identify a probable root cause.","recommendedAction":"Collect application-specific logs or add a health endpoint to narrow the investigation."}
