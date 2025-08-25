#!/usr/bin/env bash
set -euo pipefail

# Run module BDD test suites in parallel.
# Usage: scripts/run-module-bdd-parallel.sh [max-procs]
# Default max-procs = number of logical CPUs.

MAX_PROCS=${1:-$(getconf _NPROCESSORS_ONLN || echo 4)}

if ! command -v parallel >/dev/null 2>&1; then
  echo "[info] GNU parallel not found; falling back to xargs -P $MAX_PROCS"
  USE_PARALLEL=0
else
  USE_PARALLEL=1
fi

# Collect module dirs with go.mod
MODULES=$(find modules -maxdepth 1 -mindepth 1 -type d -exec test -f '{}/go.mod' \; -print | sort)

echo "[info] Detected modules:" >&2
echo "$MODULES" >&2

test_one() {
  mod="$1"
  name=$(basename "$mod")
  echo "=== BEGIN $name ===" >&2
  if (cd "$mod" && go test -count=1 -timeout=10m -run '.*BDD|.*Module' .); then
    echo "=== PASS $name ===" >&2
  else
    echo "=== FAIL $name ===" >&2
    return 1
  fi
}

export -f test_one

FAILS=0
if [ $USE_PARALLEL -eq 1 ]; then
  echo "$MODULES" | parallel -j "$MAX_PROCS" --halt soon,fail=1 test_one {}
else
  # xargs fallback
  echo "$MODULES" | xargs -I{} -P "$MAX_PROCS" bash -c 'test_one "$@"' _ {}
fi || FAILS=1

if [ $FAILS -ne 0 ]; then
  echo "[error] One or more module BDD suites failed" >&2
  exit 1
fi

echo "[info] All module BDD suites passed" >&2
