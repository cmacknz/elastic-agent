#!/usr/bin/env bash
# compare-sizes.sh — Compare endpoint-prototype packages against the original baseline.
#
# Steps performed every run:
#   1. Extract built packages from build/distributions into build/endpoint-work/raw/
#   2. Copy raw extracts to build/endpoint-work/stripped/ and remove non-endpoint files
#   3. Print total size comparison (original vs stripped) for each platform
#   4. Print per-binary breakdown for each platform, with change %
#   5. Repack each stripped directory as <name>-endpoint.{tar.gz,zip} in build/distributions
#
# Usage:
#   ./compare-sizes.sh [--no-extract]
#
#   --no-extract  Skip step 1 (re-use existing build/endpoint-work/raw/)
#                 and step 5 (skip repacking); useful for quick re-runs.
#
# Prerequisite — build the packages first:
#   EXTERNAL=true SNAPSHOT=true PACKAGES=targz,zip \
#     PLATFORMS=darwin/arm64,linux/arm64,linux/amd64,windows/amd64 mage package

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ORIGINAL_DIR="$HOME/Downloads/agent-size/original"
BUILD_DIST_DIR="$REPO_ROOT/build/distributions"
RAW_DIR="$REPO_ROOT/build/endpoint-work/raw"
STRIPPED_DIR="$REPO_ROOT/build/endpoint-work/stripped"

NO_EXTRACT=false
for arg in "$@"; do
  case "$arg" in
    --no-extract) NO_EXTRACT=true ;;
  esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

human() {
  local b=$1
  awk "BEGIN {
    if ($b >= 1073741824) printf \"%.2f GiB\", $b/1073741824
    else if ($b >= 1048576) printf \"%.2f MiB\", $b/1048576
    else if ($b >= 1024) printf \"%.2f KiB\", $b/1024
    else printf \"%d B\", $b
  }"
}

file_bytes() {
  local f="$1"
  if [[ -f "$f" ]]; then
    stat -f %z "$f" 2>/dev/null || stat -c %s "$f" 2>/dev/null || echo 0
  elif [[ -d "$f" ]]; then
    local kb
    kb=$(du -sk "$f" 2>/dev/null | awk '{print $1}')
    echo $((kb * 1024))
  else
    echo 0
  fi
}

dir_bytes() {
  local d="$1"
  [[ -d "$d" ]] || { echo 0; return; }
  local kb
  kb=$(du -sk "$d" 2>/dev/null | awk '{print $1}')
  echo $((kb * 1024))
}

# Strip everything up to and including "-SNAPSHOT-" to get e.g. "linux-x86_64".
platform_of() {
  echo "$1" | sed 's/^.*-SNAPSHOT-//'
}

# ---------------------------------------------------------------------------
# Binaries/patterns to remove for the endpoint-only build
# ---------------------------------------------------------------------------
NON_ENDPOINT_PATTERNS=(
  "apm-server*"
  "bundle.tar.gz"
  "cloudbeat*"
  "connectors*"
  "fleet-server*"
  "pf-host-agent*"
  "pf-elastic-collector*"
  "pf-elastic-symbolizer*"
  "NOTICE.pf-*"
)

strip_components_dir() {
  local dir="$1"
  [[ -d "$dir" ]] || return 0
  for pattern in "${NON_ENDPOINT_PATTERNS[@]}"; do
    # shellcheck disable=SC2086
    rm -rf "$dir"/$pattern 2>/dev/null || true
  done
}

# ---------------------------------------------------------------------------
# Step 1 — Extract packages into RAW_DIR
# ---------------------------------------------------------------------------

if [[ "$NO_EXTRACT" == false ]]; then
  echo "==> Extracting packages from $BUILD_DIST_DIR ..."
  rm -rf "$RAW_DIR"
  mkdir -p "$RAW_DIR"

  for pkg in "$BUILD_DIST_DIR"/elastic-agent-*-SNAPSHOT-*.tar.gz \
             "$BUILD_DIST_DIR"/elastic-agent-*-SNAPSHOT-*.zip; do
    [[ -f "$pkg" ]] || continue
    # Skip previously produced endpoint archives
    [[ "$pkg" == *-endpoint.* ]] && continue

    base="${pkg##*/}"
    base="${base%.tar.gz}"; base="${base%.zip}"
    # Skip elastic-agent-core-* packages (separate artifact, not the main agent).
    [[ "$base" == elastic-agent-core-* ]] && continue
    dest="$RAW_DIR/$base"

    echo "  Extracting $base ..."
    mkdir -p "$dest"
    if [[ "$pkg" == *.tar.gz ]]; then
      tar -xzf "$pkg" -C "$dest" --strip-components=1
    else
      unzip -o -q "$pkg" -d "$dest"
      inner=("$dest"/elastic-agent-*/)
      if [[ ${#inner[@]} -eq 1 && -d "${inner[0]}" ]]; then
        mv "${inner[0]}"/* "$dest"/ 2>/dev/null || true
        rmdir "${inner[0]}" 2>/dev/null || true
      fi
    fi
  done
fi

# ---------------------------------------------------------------------------
# Step 2 — Copy raw → stripped, then remove non-endpoint components
# ---------------------------------------------------------------------------

echo ""
echo "==> Stripping non-endpoint components ..."
rm -rf "$STRIPPED_DIR"
mkdir -p "$STRIPPED_DIR"

for raw_pkg_dir in "$RAW_DIR"/elastic-agent-*-SNAPSHOT-*/; do
  [[ -d "$raw_pkg_dir" ]] || continue
  name=$(basename "$raw_pkg_dir")
  strip_pkg_dir="$STRIPPED_DIR/$name"

  echo "  Copying $name ..."
  cp -a "$raw_pkg_dir" "$strip_pkg_dir"

  for comp_dir in "$strip_pkg_dir"/data/*/components/; do
    [[ -d "$comp_dir" ]] || continue
    strip_components_dir "$comp_dir"
  done
done

# ---------------------------------------------------------------------------
# Step 3 — Total size comparison per platform
# ---------------------------------------------------------------------------

echo ""
echo "==> Total size: original vs endpoint-stripped build"
printf "%-22s %12s %12s %12s %8s\n" "Platform" "Original" "Stripped" "Saved" "Change"
printf "%-22s %12s %12s %12s %8s\n" "--------" "--------" "--------" "-----" "------"

declare -a PLATFORMS_FOUND=()

for strip_dir in "$STRIPPED_DIR"/elastic-agent-*-SNAPSHOT-*/; do
  [[ -d "$strip_dir" ]] || continue
  name=$(basename "$strip_dir")
  platform=$(platform_of "$name")

  orig_dir=""
  for c in "$ORIGINAL_DIR"/elastic-agent-*-"$platform"/; do
    [[ -d "$c" ]] && orig_dir="$c" && break
  done
  [[ -z "$orig_dir" ]] && { echo "  (no original for $platform — skipping)"; continue; }

  orig_b=$(dir_bytes "$orig_dir")
  strip_b=$(dir_bytes "$strip_dir")
  saved=$(( orig_b - strip_b ))
  pct=$(awk "BEGIN { printf \"%+.1f%%\", (($strip_b - $orig_b) / $orig_b) * 100 }")

  printf "%-22s %12s %12s %12s %8s\n" \
    "$platform" "$(human $orig_b)" "$(human $strip_b)" "$(human $saved)" "$pct"
  PLATFORMS_FOUND+=("$platform")
done

# ---------------------------------------------------------------------------
# Step 4 — Per-binary breakdown for each platform
# ---------------------------------------------------------------------------

binary_breakdown() {
  local platform="$1"
  local raw_dir="$2"    # extracted (un-stripped) build
  local strip_dir="$3"  # stripped build
  local orig_dir="$4"   # original baseline

  # Locate the components/ directory in each tree.
  local raw_comp="" strip_comp="" orig_comp=""
  for d in "$raw_dir"/data/*/components/;   do [[ -d "$d" ]] && raw_comp="$d"   && break; done
  for d in "$strip_dir"/data/*/components/; do [[ -d "$d" ]] && strip_comp="$d" && break; done
  for d in "$orig_dir"/data/*/components/;  do [[ -d "$d" ]] && orig_comp="$d"  && break; done

  # Collect the sorted union of top-level entries from orig and stripped.
  # Uses sort -u rather than an associative array so this works on bash 3.2
  # (macOS default), which does not support declare -A.
  local sorted=()
  while IFS= read -r base; do
    sorted+=("$base")
  done < <(
    for src in "$orig_comp" "$strip_comp"; do
      [[ -d "$src" ]] || continue
      for entry in "$src"/*/ "$src"/*; do
        [[ -e "$entry" ]] || continue
        local b; b=$(basename "$entry")
        [[ "$b" == .* ]] && continue
        echo "$b"
      done
    done | sort -u
  )

  echo ""
  echo "==> Binary breakdown: $platform"
  printf "  %-46s %12s %12s %12s %8s\n" \
    "File" "Original" "Raw build" "Stripped" "Change"
  printf "  %-46s %12s %12s %12s %8s\n" \
    "----" "--------" "---------" "--------" "------"

  local orig_total=0 raw_total=0 strip_total=0

  local entry
  for entry in "${sorted[@]}"; do
    local orig_f="$orig_comp/$entry"
    local raw_f="$raw_comp/$entry"
    local strip_f="$strip_comp/$entry"

    local ob rb sb
    ob=$(file_bytes "$orig_f")
    rb=$(file_bytes "$raw_f")
    sb=$(file_bytes "$strip_f")

    # Skip entries that are absent everywhere (glob noise).
    (( ob + rb + sb == 0 )) && continue

    orig_total=$(( orig_total + ob ))
    raw_total=$(( raw_total + rb ))
    strip_total=$(( strip_total + sb ))

    local pct_str="—"
    if (( ob > 0 )); then
      pct_str=$(awk "BEGIN { printf \"%+.1f%%\", (($sb - $ob) / $ob) * 100 }")
    elif (( sb > 0 )); then
      pct_str="(new)"
    fi

    local ob_s rb_s sb_s
    ob_s=$( (( ob > 0 )) && human $ob || echo "—" )
    rb_s=$( (( rb > 0 )) && human $rb || echo "—" )
    sb_s=$( (( sb > 0 )) && human $sb || echo "—" )

    printf "  %-46s %12s %12s %12s %8s\n" \
      "$entry" "$ob_s" "$rb_s" "$sb_s" "$pct_str"
  done

  # Totals row for the components dir itself.
  local tot_pct
  tot_pct=$(awk "BEGIN { printf \"%+.1f%%\", (($strip_total - $orig_total) / $orig_total) * 100 }")
  printf "  %-46s %12s %12s %12s %8s\n" \
    "────────────────────── components/ total" \
    "$(human $orig_total)" "$(human $raw_total)" "$(human $strip_total)" "$tot_pct"

  # elastic-agent binary (lives outside components/, at data/<hash>/elastic-agent)
  local raw_ea_dir strip_ea_dir orig_ea_dir
  raw_ea_dir=$(dirname "$raw_comp")
  strip_ea_dir=$(dirname "$strip_comp")
  orig_ea_dir=$(dirname "$orig_comp")
  # The main agent binary is named elastic-agent on Unix and elastic-agent.exe on Windows.
  local ea_name="elastic-agent"
  [[ -f "$raw_ea_dir/elastic-agent.exe" ]] && ea_name="elastic-agent.exe"
  local ea_ob ea_rb ea_sb ea_pct
  ea_ob=$(file_bytes "$orig_ea_dir/$ea_name")
  ea_rb=$(file_bytes "$raw_ea_dir/$ea_name")
  ea_sb=$(file_bytes "$strip_ea_dir/$ea_name")
  if (( ea_ob > 0 )); then
    ea_pct=$(awk "BEGIN { printf \"%+.1f%%\", (($ea_sb - $ea_ob) / $ea_ob) * 100 }")
  elif (( ea_sb > 0 )); then
    ea_pct="(new)"
  else
    ea_pct="—"
  fi
  printf "  %-46s %12s %12s %12s %8s\n" \
    "$ea_name (main binary)" \
    "$(human $ea_ob)" "$(human $ea_rb)" "$(human $ea_sb)" "$ea_pct"
}

for platform in "${PLATFORMS_FOUND[@]}"; do
  raw_dir="" strip_dir="" orig_dir=""
  for d in "$RAW_DIR"/elastic-agent-*-SNAPSHOT-"$platform"/;      do [[ -d "$d" ]] && raw_dir="$d"   && break; done
  for d in "$STRIPPED_DIR"/elastic-agent-*-SNAPSHOT-"$platform"/; do [[ -d "$d" ]] && strip_dir="$d" && break; done
  for d in "$ORIGINAL_DIR"/elastic-agent-*-"$platform"/;          do [[ -d "$d" ]] && orig_dir="$d"  && break; done
  [[ -n "$raw_dir" && -n "$strip_dir" && -n "$orig_dir" ]] || continue
  binary_breakdown "$platform" "$raw_dir" "$strip_dir" "$orig_dir"
done

# ---------------------------------------------------------------------------
# Step 5 — Repack stripped directories into build/distributions
# ---------------------------------------------------------------------------

if [[ "$NO_EXTRACT" == false ]]; then
  echo ""
  echo "==> Repacking stripped archives into $BUILD_DIST_DIR ..."

  for strip_pkg_dir in "$STRIPPED_DIR"/elastic-agent-*-SNAPSHOT-*/; do
    [[ -d "$strip_pkg_dir" ]] || continue
    name=$(basename "$strip_pkg_dir")
    platform=$(platform_of "$name")

    if [[ "$platform" == windows-* ]]; then
      out="$BUILD_DIST_DIR/${name}-endpoint.zip"
      echo "  Packing $name → $(basename "$out") ..."
      (cd "$STRIPPED_DIR" && zip -qr "$out" "$name")
    else
      out="$BUILD_DIST_DIR/${name}-endpoint.tar.gz"
      echo "  Packing $name → $(basename "$out") ..."
      tar -czf "$out" -C "$STRIPPED_DIR" "$name"
    fi
  done

  echo ""
  echo "Endpoint archives written to $BUILD_DIST_DIR:"
  ls -lh "$BUILD_DIST_DIR"/*-endpoint.* 2>/dev/null || true
fi
