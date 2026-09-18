# Runner Mutual TLS

Production Runner communication is fail-closed and requires mutual TLS.

Runner configuration:

- XENTRA_RUNNER_TLS_CERT — server certificate.
- XENTRA_RUNNER_TLS_KEY — server private key.
- XENTRA_RUNNER_CLIENT_CA — CA used to verify control-plane client certificates.

Control-plane configuration:

- XENTRA_RUNNER_CLIENT_CERT — client certificate.
- XENTRA_RUNNER_CLIENT_KEY — client private key.
- XENTRA_RUNNER_SERVER_CA — CA used to verify Runner server certificates.

Both sides require TLS 1.3. A production control plane also rejects Runner URLs that do not use https.

Plain HTTP exists only for explicit local development and CI by setting XENTRA_RUNNER_INSECURE_DEV=true on both Runner and control-plane processes. Production deployments must not set this flag.

The Runner still exposes only typed allowlisted tools; mTLS adds transport identity and encryption rather than expanding the command surface.
