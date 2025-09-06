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
    # grep -v returns exit 1 if no lines matched (e.g. file only has mode line); tolerate that
    { grep -v '^mode:' "$f" || true; } >> "$OUT"
  fi
done

# Ensure at least one mode line exists
if ! grep -q '^mode:' "$OUT"; then
  echo 'mode: atomic' | cat - "$OUT" > "$OUT.tmp" && mv "$OUT.tmp" "$OUT"
fi

echo "Merged coverage written to $OUT" >&2
