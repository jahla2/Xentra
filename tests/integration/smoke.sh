#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
TMP=$(mktemp -d)
AI_LOG=/tmp/xentra-ai.log
RUNNER_LOG=/tmp/xentra-runner.log
CONTROL_LOG=/tmp/xentra-control.log
AI_PID=""
RUNNER_PID=""
CONTROL_PID=""

cleanup() {
  status=$?
  set +e
  if [ -n "$CONTROL_PID" ]; then kill "$CONTROL_PID" 2>/dev/null; wait "$CONTROL_PID" 2>/dev/null; fi
  if [ -n "$RUNNER_PID" ]; then kill "$RUNNER_PID" 2>/dev/null; wait "$RUNNER_PID" 2>/dev/null; fi
  if [ -n "$AI_PID" ]; then kill "$AI_PID" 2>/dev/null; wait "$AI_PID" 2>/dev/null; fi
  docker rm -f xentra-ssh-fixture >/dev/null 2>&1 || true
  if [ "$status" -ne 0 ]; then
    echo "---- AI log ----"; cat "$AI_LOG" 2>/dev/null || true
    echo "---- Runner log ----"; cat "$RUNNER_LOG" 2>/dev/null || true
    echo "---- Control-plane log ----"; cat "$CONTROL_LOG" 2>/dev/null || true
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
    exec env       XENTRA_CONTROL_ADDR=127.0.0.1:8080       XENTRA_AI_URL=http://127.0.0.1:8000       XENTRA_DATABASE_URL="$XENTRA_DATABASE_URL"       XENTRA_MASTER_KEY="$XENTRA_MASTER_KEY"       go run ./cmd/api
  ) >"$CONTROL_LOG" 2>&1 &
  CONTROL_PID=$!
  wait_http http://127.0.0.1:8080/health
}

echo "Starting AI service"
(
  cd "$ROOT/backend/ai-service"
  exec python -m uvicorn app.main:app --host 127.0.0.1 --port 8000
) >"$AI_LOG" 2>&1 &
AI_PID=$!
wait_http http://127.0.0.1:8000/health

echo "Starting runner"
(
  cd "$ROOT/backend/runner"
  exec env XENTRA_RUNNER_ADDR=127.0.0.1:8090 go run ./cmd/runner
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

echo "Connecting runner environment"
RUNNER_PAYLOAD=$(jq -n --arg url "http://127.0.0.1:8090" '{name:"CI Runner",type:"production",connectionType:"runner",runnerUrl:$url}')
RUNNER_ENV=$(request POST /api/environments "$OWNER_TOKEN" "$RUNNER_PAYLOAD")
RUNNER_ENV_ID=$(printf '%s' "$RUNNER_ENV" | jq -r '.id')
test -n "$RUNNER_ENV_ID"

echo "Connecting real SSH environment with host-key pinning"
SSH_PAYLOAD=$(jq -n --arg fp "$SSH_FINGERPRINT" --rawfile key "$TMP/id_ed25519" '{name:"CI SSH",type:"staging",connectionType:"ssh",sshHost:"127.0.0.1",sshPort:2222,sshUser:"xentra",sshHostKeyFingerprint:$fp,sshPrivateKey:$key}')
SSH_ENV=$(request POST /api/environments "$OWNER_TOKEN" "$SSH_PAYLOAD")
SSH_ENV_ID=$(printf '%s' "$SSH_ENV" | jq -r '.id')
test -n "$SSH_ENV_ID"

ENVIRONMENTS=$(request GET /api/environments "$OWNER_TOKEN" "")
test "$(printf '%s' "$ENVIRONMENTS" | jq 'length')" -eq 2

echo "Verifying bad SSH fingerprint is rejected"
BAD_SSH_PAYLOAD=$(jq -n --rawfile key "$TMP/id_ed25519" '{name:"Bad SSH",type:"staging",connectionType:"ssh",sshHost:"127.0.0.1",sshPort:2222,sshUser:"xentra",sshHostKeyFingerprint:"SHA256:not-the-host",sshPrivateKey:$key}')
BAD_STATUS=$(status_request POST /api/environments "$OWNER_TOKEN" "$BAD_SSH_PAYLOAD")
test "$BAD_STATUS" -ne 201

echo "Running evidence-backed investigation"
INVESTIGATION_PAYLOAD=$(jq -n --arg id "$RUNNER_ENV_ID" '{environmentId:$id,question:"What is running and is the environment healthy?"}')
INVESTIGATION=$(request POST /api/investigations "$OWNER_TOKEN" "$INVESTIGATION_PAYLOAD")
test -n "$(printf '%s' "$INVESTIGATION" | jq -r '.summary')"
test "$(printf '%s' "$INVESTIGATION" | jq '.evidence | length')" -ge 1

echo "Creating incident"
INCIDENT=$(request POST /api/incidents "$OWNER_TOKEN" "$INVESTIGATION_PAYLOAD")
INCIDENT_ID=$(printf '%s' "$INCIDENT" | jq -r '.id')
test -n "$INCIDENT_ID"

echo "Proposing and approving typed Docker restart"
ACTION_PAYLOAD=$(jq -n --arg inc "$INCIDENT_ID" --arg env "$RUNNER_ENV_ID" '{incidentId:$inc,environmentId:$env,action:"docker.restart",target:"xentra-ssh-fixture",reason:"integration verification"}')
ACTION=$(request POST /api/actions "$OWNER_TOKEN" "$ACTION_PAYLOAD")
ACTION_ID=$(printf '%s' "$ACTION" | jq -r '.id')
test "$(printf '%s' "$ACTION" | jq -r '.status')" = "pending_approval"

APPROVED=$(request POST "/api/actions/$ACTION_ID/approve" "$OWNER_TOKEN" "")
test "$(printf '%s' "$APPROVED" | jq -r '.status')" = "completed"
test "$(printf '%s' "$APPROVED" | jq -r '.verification.healthy')" = "true"
test "$(printf '%s' "$APPROVED" | jq -r '.approvedBy')" = "owner@example.com"

AUDIT=$(request GET /api/audit "$OWNER_TOKEN" "")
test "$(printf '%s' "$AUDIT" | jq '[.[] | select(.eventType=="action_approved")] | length')" -ge 1
test "$(printf '%s' "$AUDIT" | jq '[.[] | select(.eventType=="action_executed")] | length')" -ge 1

echo "Creating member and checking RBAC"
MEMBER_PAYLOAD='{"email":"member@example.com","password":"another-secure-password"}'
request POST /api/auth/members "$OWNER_TOKEN" "$MEMBER_PAYLOAD" >/dev/null
MEMBER_SESSION=$(request POST /api/auth/login "" '{"email":"member@example.com","password":"another-secure-password"}')
MEMBER_TOKEN=$(printf '%s' "$MEMBER_SESSION" | jq -r '.token')
MEMBER_ENVS=$(request GET /api/environments "$MEMBER_TOKEN" "")
test "$(printf '%s' "$MEMBER_ENVS" | jq 'length')" -eq 2

MEMBER_CREATE_STATUS=$(status_request POST /api/environments "$MEMBER_TOKEN" "$RUNNER_PAYLOAD")
test "$MEMBER_CREATE_STATUS" -eq 403

echo "Checking cross-organization isolation"
OTHER_SESSION=$(request POST /api/auth/register "" '{"email":"other@example.com","password":"third-secure-password","organizationName":"Other Org"}')
OTHER_TOKEN=$(printf '%s' "$OTHER_SESSION" | jq -r '.token')
OTHER_ENVS=$(request GET /api/environments "$OTHER_TOKEN" "")
test "$(printf '%s' "$OTHER_ENVS" | jq 'length')" -eq 0
CROSS_STATUS=$(status_request POST /api/investigations "$OTHER_TOKEN" "$INVESTIGATION_PAYLOAD")
test "$CROSS_STATUS" -ne 200

echo "Restarting control plane to verify PostgreSQL/session persistence"
kill "$CONTROL_PID"
wait "$CONTROL_PID" 2>/dev/null || true
CONTROL_PID=""
start_control

PERSISTED=$(request GET /api/environments "$OWNER_TOKEN" "")
test "$(printf '%s' "$PERSISTED" | jq 'length')" -eq 2

echo "Revoking owner session"
request POST /api/auth/logout "$OWNER_TOKEN" "" >/dev/null
ME_STATUS=$(status_request GET /api/auth/me "$OWNER_TOKEN" "")
test "$ME_STATUS" -eq 401

echo "Xentra integration smoke passed"
