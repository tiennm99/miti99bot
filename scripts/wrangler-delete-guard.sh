#!/usr/bin/env bash
# wrangler-delete-guard.sh — interactive CONFIRM wrapper around irreversible
# wrangler delete commands. NEVER use in CI.
#
# Usage:
#   bash scripts/wrangler-delete-guard.sh kv <namespace-id>
#   bash scripts/wrangler-delete-guard.sh d1 <database-name>
#
# Exits:
#   0  — command executed successfully
#   1  — user aborted (did not type CONFIRM)
#   2  — bad arguments
#   3  — stdin not a tty (CI safety)

set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 {kv|d1} <id-or-name>" >&2
  exit 2
fi

KIND="$1"
TARGET="$2"

case "$KIND" in
  kv)
    DESC="KV namespace $TARGET"
    CMD=(npx wrangler kv namespace delete --namespace-id "$TARGET")
    ;;
  d1)
    DESC="D1 database $TARGET"
    CMD=(npx wrangler d1 delete "$TARGET")
    ;;
  *)
    echo "unknown kind: $KIND (expected 'kv' or 'd1')" >&2
    exit 2
    ;;
esac

# Refuse to run non-interactively — prevents accidental CI execution.
if [[ ! -t 0 ]]; then
  echo "stdin not a tty — refusing to run non-interactively (CI safety)" >&2
  exit 3
fi

echo ""
echo "ABOUT TO DELETE: $DESC"
echo "This is IRREVERSIBLE. Backup files must already be on local disk."
echo ""
read -r -p "Type CONFIRM to proceed: " CONFIRM

if [[ "$CONFIRM" != "CONFIRM" ]]; then
  echo "aborted" >&2
  exit 1
fi

echo ""
echo "executing: ${CMD[*]}"
"${CMD[@]}"
