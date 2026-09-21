import unittest

from app.investigator import InvestigationEngine, investigate
from app.providers import HeuristicProvider, ProviderError
from app.schemas import Environment, Evidence, InvestigationFinding, InvestigationRequest


class FailingProvider:
    name = "failing"

    def investigate(self, request: InvestigationRequest) -> InvestigationFinding:
        raise ProviderError("provider unavailable")


class InvestigatorTests(unittest.TestCase):
    def test_identifies_unhealthy_docker_evidence(self):
        result = investigate(
            "Why is the API down?",
            [{"source": "docker.list", "success": True, "output": "api\tRestarting (1) 2 seconds ago"}],
        )
        self.assertEqual(result["confidence"], "high")
        self.assertIn("container", result["probableRootCause"].lower())

    def test_returns_low_confidence_when_no_failure_signal_exists(self):
        result = investigate(
            "Why is the API down?",
            [{"source": "system.info", "success": True, "output": "Linux prod 6.8"}],
        )
        self.assertEqual(result["confidence"], "low")

    def test_provider_failure_falls_back_to_heuristic(self):
        engine = InvestigationEngine(FailingProvider(), fallback=HeuristicProvider())
        request = InvestigationRequest(
            environment=Environment(id="env-1", name="Prod", type="production"),
            question="Why is prod down?",
            evidence=[Evidence(source="docker.list", success=True, output="api Restarting")],
        )

        result = engine.investigate(request)

        self.assertEqual(result.confidence, "high")


if __name__ == "__main__":
    unittest.main()
