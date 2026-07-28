#!/usr/bin/env bash
# build-and-compare.sh — Build the endpoint-prototype packages and compare sizes.
#
# This runs the full mage package build then calls compare-sizes.sh twice:
#   1. Raw: compare the built packages (slimmer elastic-otel-collector) vs original
#   2. Stripped: also remove non-endpoint component binaries, then compare
#
# Usage:
#   ./build-and-compare.sh [--skip-build]
#
#   --skip-build  Skip the mage package step (use existing build/distributions)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

SKIP_BUILD=false
for arg in "$@"; do
  case "$arg" in
    --skip-build) SKIP_BUILD=true ;;
  esac
done

cd "$REPO_ROOT"

if [[ "$SKIP_BUILD" == false ]]; then
  echo "==> Building endpoint-prototype packages (one platform at a time to limit memory use) ..."
  echo ""

  for platform in darwin/arm64 linux/arm64 linux/amd64 windows/amd64; do
    case "$platform" in
      windows/*) pkgfmt=zip ;;
      *)         pkgfmt=targz ;;
    esac
    echo "  Building $platform ($pkgfmt) ..."
    EXTERNAL=true SNAPSHOT=true \
      PACKAGES="$pkgfmt" \
      PLATFORMS="$platform" \
      mage package
  done

  echo ""
  echo "==> Build complete. Packages in build/distributions/:"
  ls -lh build/distributions/elastic-agent-*-SNAPSHOT-*.tar.gz \
         build/distributions/elastic-agent-*-SNAPSHOT-*.zip 2>/dev/null || true
fi

"$SCRIPT_DIR/compare-sizes.sh"

echo ""
echo "Done. See scripts/endpoint-prototype/BEATS_SUGGESTIONS.md for additional"
echo "beats repository changes that would reduce size further."
