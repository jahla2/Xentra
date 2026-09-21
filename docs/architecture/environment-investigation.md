# Environment Investigation Vertical Slice

The first Xentra product slice is deliberately read-only.

1. React creates an environment by providing the URL of an installed Xentra Runner.
2. The Go control plane asks the runner for discovery metadata and stores the environment behind a repository interface.
3. An investigation request causes the control plane to collect allowlisted evidence from the runner.
4. The control plane sends structured evidence to the Python AI service.
5. The Python investigator returns a probable root cause, confidence, and recommended next action.
6. The UI renders the finding and raw evidence.

The AI service never receives credentials and cannot execute arbitrary shell commands. The runner exposes only typed, allowlisted operations.
