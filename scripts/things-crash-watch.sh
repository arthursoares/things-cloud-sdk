#!/usr/bin/env bash
#
# things-crash-watch.sh — watch for Things.app sync crashes on macOS and
# classify them against the known sync-decode crash signatures.
#
# Run this on the machine where Things.app is signed into the SOAK TEST
# account, then run the soak tool (cmd/soak) against the same account.
# If a write ever puts undecodable data in the history, Things.app crashes
# on the next sync and this script captures and classifies the report.
#
# Usage:
#   scripts/things-crash-watch.sh            # watch crash reports + sync log
#   scripts/things-crash-watch.sh --logonly  # only stream the sync log
#   scripts/things-crash-watch.sh --scan     # classify existing crash reports and exit
#
set -euo pipefail

REPORTS_DIR="$HOME/Library/Logs/DiagnosticReports"

# Known crash sites from prior reverse-engineering (see docs/client-side-bugs.md
# and the sync-decode analysis). A crash whose backtrace hits any of these is
# the history-poisoning class this SDK is meant to prevent.
SIGNATURES=(
  "BSIdentifierFromBase58String"   # Base58 identifier decode — the leading-zero bug
  "BSSyncValueEncoder"             # Base.framework value decode
  "decodeToOneRelation"            # relation decode (heading st=0 crash)
  "insertTaskWithUUID"             # task insert
  "LegacySCHistoryPerformSync"     # Syncrony pull/apply
)

classify() {
  local file="$1"
  echo "── $(basename "$file") ($(stat -f '%Sm' "$file")) ──"
  local hit=0
  for sig in "${SIGNATURES[@]}"; do
    if grep -q "$sig" "$file" 2>/dev/null; then
      echo "  ⚠️  matches known sync-decode crash signature: $sig"
      hit=1
    fi
  done
  # Show the faulting frames regardless, for context.
  echo "  Faulting frames:"
  grep -Ei "Things|Base\.framework|Syncrony|ThingsCommon" "$file" 2>/dev/null | head -8 | sed 's/^/    /'
  if [[ $hit -eq 1 ]]; then
    echo "  VERDICT: history-poisoning crash class — inspect the last-written items."
  else
    echo "  VERDICT: crash does not match a known sync-decode signature (may be unrelated)."
  fi
  echo
}

things_reports() {
  # macOS names reports Things3-*.ips (JSON) or Things-*.crash on older systems.
  find "$REPORTS_DIR" -maxdepth 1 \( -iname 'Things*.ips' -o -iname 'Things*.crash' \) 2>/dev/null
}

if [[ "${1:-}" == "--scan" ]]; then
  found=0
  while IFS= read -r f; do [[ -n "$f" ]] && { classify "$f"; found=1; }; done < <(things_reports)
  [[ $found -eq 0 ]] && echo "No existing Things crash reports in $REPORTS_DIR."
  exit 0
fi

if [[ "${1:-}" != "--logonly" ]]; then
  echo "Watching $REPORTS_DIR for new Things crash reports…"
  # Baseline of existing reports so we only classify NEW ones.
  BASELINE="$(things_reports | sort)"
  (
    while true; do
      CURRENT="$(things_reports | sort)"
      comm -13 <(printf '%s\n' "$BASELINE") <(printf '%s\n' "$CURRENT") | while IFS= read -r f; do
        [[ -n "$f" ]] || continue
        echo; echo "🚨 NEW CRASH REPORT DETECTED"
        sleep 1   # let the OS finish writing the file
        classify "$f"
      done
      BASELINE="$CURRENT"
      sleep 5
    done
  ) &
  WATCH_PID=$!
  trap 'kill $WATCH_PID 2>/dev/null || true' EXIT
fi

echo "Streaming Things sync log (Ctrl-C to stop)…"
echo "Look for sync errors, decode failures, or the process dying mid-sync."
exec log stream \
  --style compact \
  --predicate 'processImagePath CONTAINS "Things" AND (eventMessage CONTAINS[c] "sync" OR eventMessage CONTAINS[c] "decode" OR eventMessage CONTAINS[c] "history" OR eventMessage CONTAINS[c] "base58" OR category == "Syncrony")'
