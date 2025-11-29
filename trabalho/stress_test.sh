#!/usr/bin/env bash

set -euo pipefail

HOST=${1:-127.0.0.1}
PORT=${2:-5000}
REQUESTS=${3:-2000}
MESSAGE=${4:-"stress-test"}
CONCURRENCY=${5:-200}

echo "Starting stress test against ${HOST}:${PORT}"
echo "Total requests: ${REQUESTS}, concurrency: ${CONCURRENCY}"

active_jobs=0

cleanup() {
  if jobs -p >/dev/null 2>&1; then
    kill $(jobs -p) 2>/dev/null || true
  fi
}

trap cleanup EXIT

for i in $(seq 1 "${REQUESTS}"); do
  (
    printf "%s #%d\n" "${MESSAGE}" "${i}" | nc "${HOST}" "${PORT}" >/dev/null
  ) &
  active_jobs=$((active_jobs + 1))
  if (( active_jobs >= CONCURRENCY )); then
    wait -n
    active_jobs=$((active_jobs - 1))
  fi
done

wait
echo "Stress test completed."

