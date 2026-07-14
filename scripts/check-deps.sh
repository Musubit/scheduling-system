#!/usr/bin/env bash
set -e

FORBIDDEN=(
  "scheduling-system/backend/services"
  "scheduling-system/backend/database"
  "scheduling-system/backend/wails"
)

FAIL=0
for pkg in $(go list ./backend/scheduling/... 2>/dev/null); do
  deps=$(go list -deps -f '{{.ImportPath}}' "$pkg")
  for forbidden in "${FORBIDDEN[@]}"; do
    if echo "$deps" | grep -q "^${forbidden}"; then
      echo "❌ $pkg imports $forbidden"
      FAIL=1
    fi
  done
done

if [ $FAIL -eq 0 ]; then
  echo "✅ backend/scheduling/* 依赖边界正常"
else
  exit 1
fi