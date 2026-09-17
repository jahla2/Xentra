import unittest
from app.investigator import investigate

class InvestigatorTests(unittest.TestCase):
    def test_identifies_unhealthy_docker_evidence(self):
        result = investigate("Why is the API down?", [{"source":"docker.list","success":True,"output":"api\tRestarting (1) 2 seconds ago"}])
        self.assertEqual(result["confidence"], "high")
        self.assertIn("container", result["probableRootCause"].lower())

    def test_returns_low_confidence_when_no_failure_signal_exists(self):
        result = investigate("Why is the API down?", [{"source":"system.info","success":True,"output":"Linux prod 6.8"}])
        self.assertEqual(result["confidence"], "low")

if __name__ == "__main__": unittest.main()
