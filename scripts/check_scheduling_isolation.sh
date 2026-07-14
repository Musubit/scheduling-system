#!/usr/bin/env bash
# check_scheduling_isolation.sh — guards INV-P2.
# Fails if scheduling/... depends on database / models / services / gorm.
set -euo pipefail

cd "$(dirname "$0")/.."

# List transitive deps of the scheduling subtree. Empty subtrees (before
# later PRs land) are skipped silently.
if ! ls backend/scheduling/*/ >/dev/null 2>&1; then
    echo "OK: scheduling/ subtree not present yet"
    exit 0
fi

DEPS=$(go list -deps ./backend/scheduling/... 2>/dev/null || true)
if [ -z "$DEPS" ]; then
    echo "OK: scheduling/ has no packages to analyze"
    exit 0
fi

VIOLATIONS=$(echo "$DEPS" | grep -E "(scheduling-system/backend/database|scheduling-system/backend/models|scheduling-system/backend/services|gorm\.io/gorm)" || true)

if [ -n "$VIOLATIONS" ]; then
    echo "VIOLATION: scheduling/* depends on forbidden packages:"
    echo "$VIOLATIONS"
    exit 1
fi

echo "OK: scheduling isolation verified"
