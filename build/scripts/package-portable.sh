#!/usr/bin/env bash
# package-portable.sh — Linux portable tar.gz packager, mirror of
# package-portable.ps1 (Windows). Bundles the Wails app binary + optional
# scheduler binary into a single self-contained archive that unpacks and
# runs from any directory (no /usr/local install required).
#
# Usage: package-portable.sh --bin-dir bin --app-name scheduling-system --version 0.5.5

set -euo pipefail

BIN_DIR="bin"
APP_NAME="scheduling-system"
VERSION="0.5.5"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin-dir)  BIN_DIR="$2"; shift 2 ;;
    --app-name) APP_NAME="$2"; shift 2 ;;
    --version)  VERSION="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

MAIN_BIN="${BIN_DIR}/${APP_NAME}"
SCHEDULER_SRC="scheduler/dist/scheduler"
STAGE_DIR="${BIN_DIR}/${APP_NAME}-portable-v${VERSION}"
ARCHIVE="${BIN_DIR}/${APP_NAME}-portable-v${VERSION}-linux-$(uname -m).tar.gz"

if [[ ! -f "$MAIN_BIN" ]]; then
  echo "Main binary not found: $MAIN_BIN" >&2
  echo "Run 'task linux:build' first." >&2
  exit 1
fi

# Fresh stage dir
rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"
cp -f "$MAIN_BIN" "$STAGE_DIR/${APP_NAME}"
chmod +x "$STAGE_DIR/${APP_NAME}"

HAS_SCHEDULER=false
if [[ -f "$SCHEDULER_SRC" ]]; then
  mkdir -p "$STAGE_DIR/scheduler"
  cp -f "$SCHEDULER_SRC" "$STAGE_DIR/scheduler/scheduler"
  chmod +x "$STAGE_DIR/scheduler/scheduler"
  HAS_SCHEDULER=true
  echo "Scheduler bundled: scheduler/scheduler"
else
  echo "Scheduler not found at $SCHEDULER_SRC — skipping (SA-only mode)"
fi

# Reproducible-ish tarball: sort entries + strip owner metadata.
tar --sort=name --owner=0 --group=0 --numeric-owner \
    -czf "$ARCHIVE" -C "$BIN_DIR" "$(basename "$STAGE_DIR")"

rm -rf "$STAGE_DIR"

echo "======================================"
echo "Portable tar.gz created: $ARCHIVE"
echo "Contents:"
echo "  - ${APP_NAME}"
if $HAS_SCHEDULER; then
  echo "  - scheduler/scheduler  (OR-Tools solver)"
fi
echo ""
echo "To use: tar xzf $(basename "$ARCHIVE") && cd $(basename "$STAGE_DIR") && ./${APP_NAME}"
echo "======================================"
