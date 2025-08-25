#!/usr/bin/env bash
set -euo pipefail
# Merge multiple Go coverage profiles into one.
# Usage: scripts/merge-coverprofiles.sh output profile1 [profile2 ...]
# Lines starting with 'mode:' only kept from first file.

if [ $# -lt 2 ]; then
  echo "Usage: $0 output merged1 [merged2 ...]" >&2
  exit 1
fi

OUT=$1; shift
FIRST=1
true > "$OUT"
for f in "$@"; do
  [ -f "$f" ] || { echo "[warn] Missing profile $f" >&2; continue; }
  if [ $FIRST -eq 1 ]; then
    cat "$f" >> "$OUT"
    FIRST=0
  else
    grep -v '^mode:' "$f" >> "$OUT"
  fi
done

echo "Merged coverage written to $OUT" >&2
