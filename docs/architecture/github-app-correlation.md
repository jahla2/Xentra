# GitHub App integration and repository correlation

GitHub App authentication is the preferred production integration mode.

## Authentication modes

### github_app (default)

The control plane is configured with:

- `XENTRA_GITHUB_APP_ID`
- `XENTRA_GITHUB_APP_PRIVATE_KEY` or `XENTRA_GITHUB_APP_PRIVATE_KEY_FILE`
- optional `XENTRA_GITHUB_API_URL` for test/GitHub Enterprise-compatible endpoints

When an owner connects a repository, Xentra:

1. signs a short-lived RS256 GitHub App JWT;
2. resolves the GitHub App installation for the selected repository;
3. stores the installation ID, not an installation access token;
4. exchanges the App JWT for a short-lived installation token when repository data is needed;
5. caches the installation token until five minutes before expiry.

### token (fallback)

A personal/access token can still be supplied explicitly. It is encrypted with the normal Xentra credential store and is never returned to the UI.

## Incident correlation

Before an incident diagnosis, Xentra fetches repository context for the connected environment:

- recent commits;
- recent workflow runs;
- the most recent failed workflow run (preferred correlation target);
- the failed run's head SHA and branch;
- commit detail and changed files;
- bounded patch snippets (3,000 characters per file, 12,000 total);
- pull requests associated with the correlated commit.

Repository evidence is passed through the same control-plane redaction pipeline as infrastructure evidence before reaching the AI service.

The incident timeline contains normalized commit, workflow, changed-file and pull-request events.

## Safety

Repository correlation is read-only. GitHub App authentication never grants the investigation loop permission to mutate infrastructure, merge code or approve remediation actions.
