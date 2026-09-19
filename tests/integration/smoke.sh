#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
TMP=$(mktemp -d)
AI_LOG=/tmp/xentra-ai.log
RUNNER_LOG=/tmp/xentra-runner.log
OUTBOUND_RUNNER_LOG=/tmp/xentra-outbound-runner.log
CONTROL_LOG=/tmp/xentra-control.log
OTEL_LOG=/tmp/xentra-otel.jsonl
AI_PID=""
RUNNER_PID=""
OUTBOUND_RUNNER_PID=""
CONTROL_PID=""
OTEL_PID=""

cleanup() {
  status=$?
  set +e
  if [ -n "$CONTROL_PID" ]; then kill "$CONTROL_PID" 2>/dev/null; wait "$CONTROL_PID" 2>/dev/null; fi
  if [ -n "$RUNNER_PID" ]; then kill "$RUNNER_PID" 2>/dev/null; wait "$RUNNER_PID" 2>/dev/null; fi
  if [ -n "$OUTBOUND_RUNNER_PID" ]; then kill "$OUTBOUND_RUNNER_PID" 2>/dev/null; wait "$OUTBOUND_RUNNER_PID" 2>/dev/null; fi
  if [ -n "$AI_PID" ]; then kill "$AI_PID" 2>/dev/null; wait "$AI_PID" 2>/dev/null; fi
  if [ -n "$OTEL_PID" ]; then kill "$OTEL_PID" 2>/dev/null; wait "$OTEL_PID" 2>/dev/null; fi
  docker rm -f xentra-ssh-fixture >/dev/null 2>&1 || true
  if [ "$status" -ne 0 ]; then
    echo "---- AI log ----"; cat "$AI_LOG" 2>/dev/null || true
    echo "---- Legacy Runner log ----"; cat "$RUNNER_LOG" 2>/dev/null || true
    echo "---- Outbound Runner log ----"; cat "$OUTBOUND_RUNNER_LOG" 2>/dev/null || true
    echo "---- Control-plane log ----"; cat "$CONTROL_LOG" 2>/dev/null || true
    echo "---- OTEL spans ----"; cat "$OTEL_LOG" 2>/dev/null || true
  fi
  rm -rf "$TMP"
  trap - EXIT
  exit "$status"
}
trap cleanup EXIT

wait_http() {
  url=$1
  for _ in $(seq 1 90); do
    if curl -fsS "$url" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  echo "timed out waiting for $url" >&2
  return 1
}

generate_runner_mtls() {
  openssl req -x509 -newkey rsa:2048 -nodes     -keyout "$TMP/runner-ca.key"     -out "$TMP/runner-ca.crt"     -subj "/CN=Xentra Integration Runner CA"     -days 1 >/dev/null 2>&1

  openssl req -newkey rsa:2048 -nodes     -keyout "$TMP/runner-control.key"     -out "$TMP/runner-control.csr"     -subj "/CN=127.0.0.1" >/dev/null 2>&1
  printf '%s\n' 'subjectAltName=IP:127.0.0.1' 'extendedKeyUsage=serverAuth' > "$TMP/runner-control.ext"
  openssl x509 -req     -in "$TMP/runner-control.csr"     -CA "$TMP/runner-ca.crt"     -CAkey "$TMP/runner-ca.key"     -CAcreateserial     -out "$TMP/runner-control.crt"     -days 1     -extfile "$TMP/runner-control.ext" >/dev/null 2>&1

  openssl req -newkey rsa:2048 -nodes     -keyout "$TMP/runner-client.key"     -out "$TMP/runner-client.csr"     -subj "/CN=xentra-ci-runner" >/dev/null 2>&1
  printf '%s\n' 'extendedKeyUsage=clientAuth' > "$TMP/runner-client.ext"
  openssl x509 -req     -in "$TMP/runner-client.csr"     -CA "$TMP/runner-ca.crt"     -CAkey "$TMP/runner-ca.key"     -CAserial "$TMP/runner-ca.srl"     -out "$TMP/runner-client.crt"     -days 1     -extfile "$TMP/runner-client.ext" >/dev/null 2>&1
}

wait_runner_control() {
  for _ in $(seq 1 90); do
    if curl -fsS       --cert "$TMP/runner-client.crt"       --key "$TMP/runner-client.key"       --cacert "$TMP/runner-ca.crt"       https://127.0.0.1:8081/health >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for mTLS Runner control" >&2
  return 1
}

request() {
  method=$1
  path=$2
  token=$3
  data=$4
  if [ -n "$token" ]; then
    if [ -n "$data" ]; then
      curl -fsS -X "$method" "http://127.0.0.1:8080$path" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$data"
    else
      curl -fsS -X "$method" "http://127.0.0.1:8080$path" -H "Authorization: Bearer $token"
    fi
  else
    if [ -n "$data" ]; then
      curl -fsS -X "$method" "http://127.0.0.1:8080$path" -H "Content-Type: application/json" -d "$data"
    else
      curl -fsS -X "$method" "http://127.0.0.1:8080$path"
    fi
  fi
}

status_request() {
  method=$1
  path=$2
  token=$3
  data=$4
  if [ -n "$token" ]; then
    curl -sS -o "$TMP/status-body.json" -w "%{http_code}" -X "$method" "http://127.0.0.1:8080$path" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$data"
  else
    curl -sS -o "$TMP/status-body.json" -w "%{http_code}" -X "$method" "http://127.0.0.1:8080$path" -H "Content-Type: application/json" -d "$data"
  fi
}

start_control() {
  (
    cd "$ROOT/backend/control-plane"
    exec env       XENTRA_CONTROL_ADDR=127.0.0.1:8080       XENTRA_RUNNER_CONTROL_ADDR=127.0.0.1:8081       XENTRA_RUNNER_CONTROL_TLS_CERT="$TMP/runner-control.crt"       XENTRA_RUNNER_CONTROL_TLS_KEY="$TMP/runner-control.key"       XENTRA_RUNNER_CONTROL_CLIENT_CA="$TMP/runner-ca.crt"       XENTRA_AI_URL=http://127.0.0.1:8000       XENTRA_DATABASE_URL="$XENTRA_DATABASE_URL"       XENTRA_MASTER_KEY="$XENTRA_MASTER_KEY"       XENTRA_RUNNER_INSECURE_DEV=true       OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318       OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf       go run ./cmd/api
  ) >"$CONTROL_LOG" 2>&1 &
  CONTROL_PID=$!
  wait_http http://127.0.0.1:8080/health
  wait_runner_control
}

echo "Starting OTLP trace sink"
cat >"$TMP/otel_sink.py" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from opentelemetry.proto.collector.trace.v1.trace_service_pb2 import (
    ExportTraceServiceRequest,
    ExportTraceServiceResponse,
)

output = sys.argv[1]

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.end_headers()
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        if self.path != "/v1/traces":
            self.send_response(404)
            self.end_headers()
            return
        length = int(self.headers.get("Content-Length", "0"))
        request = ExportTraceServiceRequest()
        request.ParseFromString(self.rfile.read(length))
        with open(output, "a", encoding="utf-8") as handle:
            for resource_spans in request.resource_spans:
                service = "unknown"
                for item in resource_spans.resource.attributes:
                    if item.key == "service.name":
                        service = item.value.string_value
                        break
                for scope_spans in resource_spans.scope_spans:
                    for span in scope_spans.spans:
                        handle.write(json.dumps({
                            "service": service,
                            "trace_id": span.trace_id.hex(),
                            "span_id": span.span_id.hex(),
                            "parent_span_id": span.parent_span_id.hex(),
                            "name": span.name,
                        }) + "\n")
                handle.flush()
        response = ExportTraceServiceResponse().SerializeToString()
        self.send_response(200)
        self.send_header("Content-Type", "application/x-protobuf")
        self.send_header("Content-Length", str(len(response)))
        self.end_headers()
        self.wfile.write(response)

    def log_message(self, *_):
        pass

ThreadingHTTPServer(("127.0.0.1", 4318), Handler).serve_forever()
PY
: >"$OTEL_LOG"
python "$TMP/otel_sink.py" "$OTEL_LOG" >"$TMP/otel-sink.log" 2>&1 &
OTEL_PID=$!
wait_http http://127.0.0.1:4318/health

echo "Generating Runner-control mTLS certificates"
generate_runner_mtls

echo "Starting AI service"
(
  cd "$ROOT/backend/ai-service"
  exec env OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf python -m uvicorn app.main:app --host 127.0.0.1 --port 8000
) >"$AI_LOG" 2>&1 &
AI_PID=$!
wait_http http://127.0.0.1:8000/health

echo "Starting runner"
(
  cd "$ROOT/backend/runner"
  exec env XENTRA_RUNNER_ADDR=127.0.0.1:8090 XENTRA_RUNNER_INSECURE_DEV=true go run ./cmd/runner
) >"$RUNNER_LOG" 2>&1 &
RUNNER_PID=$!
wait_http http://127.0.0.1:8090/health

echo "Building disposable SSH host"
ssh-keygen -q -t ed25519 -N "" -f "$TMP/id_ed25519"
AUTHORIZED_KEY=$(cat "$TMP/id_ed25519.pub")
docker build -q -t xentra-ssh-fixture "$ROOT/tests/integration/ssh-fixture" >/dev/null
docker run -d --name xentra-ssh-fixture -p 127.0.0.1:2222:22 -e "AUTHORIZED_KEY=$AUTHORIZED_KEY" xentra-ssh-fixture >/dev/null

for _ in $(seq 1 30); do
  if ssh-keyscan -t ed25519 -p 2222 127.0.0.1 >"$TMP/hostkey" 2>/dev/null; then break; fi
  sleep 1
done
test -s "$TMP/hostkey"
SSH_FINGERPRINT=$(ssh-keygen -lf "$TMP/hostkey" -E sha256 | awk 'NR==1 {print $2}')

echo "Starting PostgreSQL-backed control plane"
start_control

echo "Registering owner organization"
OWNER_SESSION=$(request POST /api/auth/register "" '{"email":"owner@example.com","password":"very-secure-password","organizationName":"Integration Org"}')
OWNER_TOKEN=$(printf '%s' "$OWNER_SESSION" | jq -r '.token')
ORG_ID=$(printf '%s' "$OWNER_SESSION" | jq -r '.principal.organizationId')
test -n "$OWNER_TOKEN"
test -n "$ORG_ID"

echo "Creating project"
PROJECT=$(request POST /api/projects "$OWNER_TOKEN" '{"name":"Core Platform","description":"Integration project"}')
PROJECT_ID=$(printf '%s' "$PROJECT" | jq -r '.id')
test -n "$PROJECT_ID"
PROJECTS=$(request GET /api/projects "$OWNER_TOKEN" "")
test "$(printf '%s' "$PROJECTS" | jq 'length')" -eq 1

echo "Creating outbound Runner enrollment"
OUTBOUND_ENROLLMENT=$(request POST /api/runner-enrollments "$OWNER_TOKEN" "$(jq -n --arg project "$PROJECT_ID" '{projectId:$project,name:"CI Outbound Runner",type:"production"}')")
OUTBOUND_ENV_ID=$(printf '%s' "$OUTBOUND_ENROLLMENT" | jq -r '.environment.id')
OUTBOUND_RUNNER_ID=$(printf '%s' "$OUTBOUND_ENROLLMENT" | jq -r '.runnerId')
OUTBOUND_RUNNER_TOKEN=$(printf '%s' "$OUTBOUND_ENROLLMENT" | jq -r '.runnerToken')
test -n "$OUTBOUND_ENV_ID"
test -n "$OUTBOUND_RUNNER_ID"
test -n "$OUTBOUND_RUNNER_TOKEN"
test "$(printf '%s' "$OUTBOUND_ENROLLMENT" | jq -r '.environment.connectionType')" = "runner_outbound"

echo "Starting outbound Runner agent"
(
  cd "$ROOT/backend/runner"
  exec env \
    XENTRA_CONTROL_URL=https://127.0.0.1:8081 \
    XENTRA_RUNNER_ID="$OUTBOUND_RUNNER_ID" \
    XENTRA_RUNNER_TOKEN="$OUTBOUND_RUNNER_TOKEN" \
    XENTRA_CONTROL_CLIENT_CERT="$TMP/runner-client.crt" \
    XENTRA_CONTROL_CLIENT_KEY="$TMP/runner-client.key" \
    XENTRA_CONTROL_SERVER_CA="$TMP/runner-ca.crt" \
    XENTRA_CONTROL_POLL_SECONDS=1 \
    OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 \
    OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf \
    go run ./cmd/runner
) >"$OUTBOUND_RUNNER_LOG" 2>&1 &
OUTBOUND_RUNNER_PID=$!

echo "Waiting for outbound Runner check-in"
OUTBOUND_HOST=""
for _ in $(seq 1 90); do
  ENVIRONMENTS=$(request GET /api/environments "$OWNER_TOKEN" "")
  OUTBOUND_HOST=$(printf '%s' "$ENVIRONMENTS" | jq -r --arg id "$OUTBOUND_ENV_ID" '.[] | select(.id==$id) | .hostname')
  if [ -n "$OUTBOUND_HOST" ] && [ "$OUTBOUND_HOST" != "null" ]; then
    break
  fi
  sleep 1
done
test -n "$OUTBOUND_HOST"
OUTBOUND_CPU=$(printf '%s' "$ENVIRONMENTS" | jq -r --arg id "$OUTBOUND_ENV_ID" '.[] | select(.id==$id) | .cpu')
OUTBOUND_MEMORY=$(printf '%s' "$ENVIRONMENTS" | jq -r --arg id "$OUTBOUND_ENV_ID" '.[] | select(.id==$id) | .memory')
OUTBOUND_DISK=$(printf '%s' "$ENVIRONMENTS" | jq -r --arg id "$OUTBOUND_ENV_ID" '.[] | select(.id==$id) | .disk')
OUTBOUND_CONTAINERS=$(printf '%s' "$ENVIRONMENTS" | jq --arg id "$OUTBOUND_ENV_ID" '[.[] | select(.id==$id) | .containers[]] | length')
test -n "$OUTBOUND_CPU"
test -n "$OUTBOUND_MEMORY"
test -n "$OUTBOUND_DISK"
test "$OUTBOUND_CONTAINERS" -ge 1

echo "Connecting legacy runner environment"
RUNNER_PAYLOAD=$(jq -n --arg project "$PROJECT_ID" --arg url "http://127.0.0.1:8090" '{projectId:$project,name:"CI Runner",type:"production",connectionType:"runner",runnerUrl:$url}')
RUNNER_ENV=$(request POST /api/environments "$OWNER_TOKEN" "$RUNNER_PAYLOAD")
RUNNER_ENV_ID=$(printf '%s' "$RUNNER_ENV" | jq -r '.id')
test -n "$RUNNER_ENV_ID"

echo "Connecting real SSH environment with host-key pinning"
SSH_PAYLOAD=$(jq -n --arg project "$PROJECT_ID" --arg fp "$SSH_FINGERPRINT" --rawfile key "$TMP/id_ed25519" '{projectId:$project,name:"CI SSH",type:"staging",connectionType:"ssh",sshHost:"127.0.0.1",sshPort:2222,sshUser:"xentra",sshHostKeyFingerprint:$fp,sshPrivateKey:$key}')
SSH_ENV=$(request POST /api/environments "$OWNER_TOKEN" "$SSH_PAYLOAD")
SSH_ENV_ID=$(printf '%s' "$SSH_ENV" | jq -r '.id')
test -n "$SSH_ENV_ID"

ENVIRONMENTS=$(request GET /api/environments "$OWNER_TOKEN" "")
test "$(printf '%s' "$ENVIRONMENTS" | jq 'length')" -eq 3

echo "Verifying bad SSH fingerprint is rejected"
BAD_SSH_PAYLOAD=$(jq -n --arg project "$PROJECT_ID" --rawfile key "$TMP/id_ed25519" '{projectId:$project,name:"Bad SSH",type:"staging",connectionType:"ssh",sshHost:"127.0.0.1",sshPort:2222,sshUser:"xentra",sshHostKeyFingerprint:"SHA256:not-the-host",sshPrivateKey:$key}')
BAD_STATUS=$(status_request POST /api/environments "$OWNER_TOKEN" "$BAD_SSH_PAYLOAD")
test "$BAD_STATUS" -ne 201

echo "Running evidence-backed investigation"
INVESTIGATION_PAYLOAD=$(jq -n --arg id "$OUTBOUND_ENV_ID" '{environmentId:$id,question:"What is running and is the environment healthy?"}')
INVESTIGATION=$(request POST /api/investigations "$OWNER_TOKEN" "$INVESTIGATION_PAYLOAD")
test -n "$(printf '%s' "$INVESTIGATION" | jq -r '.summary')"
test "$(printf '%s' "$INVESTIGATION" | jq '.evidence | length')" -ge 1

echo "Creating incident"
INCIDENT=$(request POST /api/incidents "$OWNER_TOKEN" "$INVESTIGATION_PAYLOAD")
INCIDENT_ID=$(printf '%s' "$INCIDENT" | jq -r '.id')
test -n "$INCIDENT_ID"

echo "Proposing and rejecting typed Docker restart"
REJECT_PAYLOAD=$(jq -n --arg inc "$INCIDENT_ID" --arg env "$OUTBOUND_ENV_ID" '{incidentId:$inc,environmentId:$env,action:"docker.restart",target:"xentra-ssh-fixture",reason:"rejection verification"}')
REJECT_ACTION=$(request POST /api/actions "$OWNER_TOKEN" "$REJECT_PAYLOAD")
REJECT_ACTION_ID=$(printf '%s' "$REJECT_ACTION" | jq -r '.id')
test "$(printf '%s' "$REJECT_ACTION" | jq -r '.status')" = "pending_approval"
REJECTED=$(request POST "/api/actions/$REJECT_ACTION_ID/reject" "$OWNER_TOKEN" "")
test "$(printf '%s' "$REJECTED" | jq -r '.status')" = "rejected"
test "$(printf '%s' "$REJECTED" | jq -r '.rejectedBy')" = "owner@example.com"
test "$(docker inspect -f '{{.State.Running}}' xentra-ssh-fixture)" = "true"

echo "Proposing and approving typed Docker restart"
ACTION_PAYLOAD=$(jq -n --arg inc "$INCIDENT_ID" --arg env "$OUTBOUND_ENV_ID" '{incidentId:$inc,environmentId:$env,action:"docker.restart",target:"xentra-ssh-fixture",reason:"integration verification"}')
ACTION=$(request POST /api/actions "$OWNER_TOKEN" "$ACTION_PAYLOAD")
ACTION_ID=$(printf '%s' "$ACTION" | jq -r '.id')
test "$(printf '%s' "$ACTION" | jq -r '.status')" = "pending_approval"

APPROVED=$(request POST "/api/actions/$ACTION_ID/approve" "$OWNER_TOKEN" "")
test "$(printf '%s' "$APPROVED" | jq -r '.status')" = "approved"
test "$(printf '%s' "$APPROVED" | jq -r '.executionStage')" = "queued"
test "$(printf '%s' "$APPROVED" | jq -r '.approvedBy')" = "owner@example.com"

echo "Polling persisted live execution state"
CURRENT_ACTION="$APPROVED"
for _ in $(seq 1 80); do
  CURRENT_ACTION=$(request GET "/api/actions/$ACTION_ID" "$OWNER_TOKEN" "")
  ACTION_STATUS=$(printf '%s' "$CURRENT_ACTION" | jq -r '.status')
  if [ "$ACTION_STATUS" = "completed" ] || [ "$ACTION_STATUS" = "failed" ] || [ "$ACTION_STATUS" = "verification_failed" ]; then
    break
  fi
  sleep 0.25
done
test "$(printf '%s' "$CURRENT_ACTION" | jq -r '.status')" = "completed"
test "$(printf '%s' "$CURRENT_ACTION" | jq -r '.executionStage')" = "completed"
test "$(printf '%s' "$CURRENT_ACTION" | jq -r '.verification.healthy')" = "true"
test "$(printf '%s' "$CURRENT_ACTION" | jq -r '.durationMs')" -ge 0

AUDIT=$(request GET /api/audit "$OWNER_TOKEN" "")
test "$(printf '%s' "$AUDIT" | jq '[.[] | select(.eventType=="action_rejected")] | length')" -ge 1
test "$(printf '%s' "$AUDIT" | jq '[.[] | select(.eventType=="action_approved")] | length')" -ge 1
test "$(printf '%s' "$AUDIT" | jq '[.[] | select(.eventType=="action_executed" and .tool=="docker.restart" and .target=="xentra-ssh-fixture" and .approval=="approved")] | length')" -ge 1

echo "Creating member and checking RBAC"
MEMBER_PAYLOAD='{"email":"member@example.com","password":"another-secure-password"}'
request POST /api/auth/members "$OWNER_TOKEN" "$MEMBER_PAYLOAD" >/dev/null
MEMBER_SESSION=$(request POST /api/auth/login "" '{"email":"member@example.com","password":"another-secure-password"}')
MEMBER_TOKEN=$(printf '%s' "$MEMBER_SESSION" | jq -r '.token')
MEMBER_PROJECTS=$(request GET /api/projects "$MEMBER_TOKEN" "")
test "$(printf '%s' "$MEMBER_PROJECTS" | jq 'length')" -eq 1
MEMBER_ENVS=$(request GET /api/environments "$MEMBER_TOKEN" "")
test "$(printf '%s' "$MEMBER_ENVS" | jq 'length')" -eq 3

MEMBER_CREATE_STATUS=$(status_request POST /api/environments "$MEMBER_TOKEN" "$RUNNER_PAYLOAD")
test "$MEMBER_CREATE_STATUS" -eq 403

echo "Checking cross-organization isolation"
OTHER_SESSION=$(request POST /api/auth/register "" '{"email":"other@example.com","password":"third-secure-password","organizationName":"Other Org"}')
OTHER_TOKEN=$(printf '%s' "$OTHER_SESSION" | jq -r '.token')
OTHER_PROJECTS=$(request GET /api/projects "$OTHER_TOKEN" "")
test "$(printf '%s' "$OTHER_PROJECTS" | jq 'length')" -eq 0
OTHER_ENVS=$(request GET /api/environments "$OTHER_TOKEN" "")
test "$(printf '%s' "$OTHER_ENVS" | jq 'length')" -eq 0
CROSS_STATUS=$(status_request POST /api/investigations "$OTHER_TOKEN" "$INVESTIGATION_PAYLOAD")
test "$CROSS_STATUS" -ne 200

echo "Restarting control plane to verify PostgreSQL/session persistence"
kill "$CONTROL_PID"
wait "$CONTROL_PID" 2>/dev/null || true
CONTROL_PID=""
start_control

PERSISTED_PROJECTS=$(request GET /api/projects "$OWNER_TOKEN" "")
test "$(printf '%s' "$PERSISTED_PROJECTS" | jq 'length')" -eq 1
PERSISTED=$(request GET /api/environments "$OWNER_TOKEN" "")
test "$(printf '%s' "$PERSISTED" | jq 'length')" -eq 3
test -n "$(printf '%s' "$PERSISTED" | jq -r --arg id "$OUTBOUND_ENV_ID" '.[] | select(.id==$id) | .cpu')"
test "$(printf '%s' "$PERSISTED" | jq --arg id "$OUTBOUND_ENV_ID" '[.[] | select(.id==$id) | .containers[]] | length')" -ge 1

echo "Verifying outbound Runner reconnects after control-plane restart"
POST_RESTART_INVESTIGATION=$(request POST /api/investigations "$OWNER_TOKEN" "$INVESTIGATION_PAYLOAD")
test -n "$(printf '%s' "$POST_RESTART_INVESTIGATION" | jq -r '.summary')"
test "$(printf '%s' "$POST_RESTART_INVESTIGATION" | jq '.evidence | length')" -ge 1

echo "Verifying distributed trace crosses control plane, AI service, and outbound Runner"
TRACE_OK=""
for _ in $(seq 1 20); do
  if python - "$OTEL_LOG" <<'PY'
import json
import sys
from collections import defaultdict

by_trace = defaultdict(set)
with open(sys.argv[1], encoding="utf-8") as handle:
    for line in handle:
        if not line.strip():
            continue
        item = json.loads(line)
        by_trace[item["trace_id"]].add(item["service"])

required = {"xentra-control-plane", "xentra-ai-service", "xentra-runner"}
matches = [trace_id for trace_id, services in by_trace.items() if required.issubset(services)]
if not matches:
    raise SystemExit(1)
print(matches[0])
PY
  then
    TRACE_OK=1
    break
  fi
  sleep 1
done
test -n "$TRACE_OK"

echo "Revoking owner session"
request POST /api/auth/logout "$OWNER_TOKEN" "" >/dev/null
ME_STATUS=$(status_request GET /api/auth/me "$OWNER_TOKEN" "")
test "$ME_STATUS" -eq 401

echo "Xentra integration smoke passed"
