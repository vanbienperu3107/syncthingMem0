#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_DIR"

GOOS_LIST="${GOOS_LIST:-$(go env GOOS)}"
GOARCH_LIST="${GOARCH_LIST:-$(go env GOARCH)}"
OUT_DIR="${OUT_DIR:-./dist/brands}"
MODE="${1:-both}"
INCLUDE_INFRA="${INCLUDE_INFRA:-false}"

if [[ "$MODE" != "client" && "$MODE" != "server" && "$MODE" != "both" ]]; then
  echo "Usage: $0 [client|server|both]"
  echo "Optional env:"
  echo "  GOOS_LIST=linux,windows,darwin"
  echo "  GOARCH_LIST=amd64,arm64,386"
  echo "  INCLUDE_INFRA=true (only for server to include crash/upgrade/metrics services)"
  echo "  OUT_DIR=./dist/brands"
  exit 1
fi

IFS=',' read -r -a GOOS_ARR <<< "$GOOS_LIST"
IFS=',' read -r -a GOARCH_ARR <<< "$GOARCH_LIST"

build_one() {
  local target="$1"
  local goos="$2"
  local goarch="$3"
  local out_dir="$4"

  rm -f "${target}-"*"-${goos}-${goarch}"*.zip || true
  go run build.go -goos "$goos" -goarch "$goarch" zip "$target" >/dev/null
  local artifact
  artifact="$(ls "${target}-"*"-${goos}-${goarch}"*.zip | head -n 1)"

  if [[ ! -f "$artifact" ]]; then
    echo "Build failed for $target on $goos/$goarch"
    exit 1
  fi

  mkdir -p "$out_dir"
  mv "$artifact" "$out_dir/$artifact"
  echo "[OK] $target -> $out_dir/$artifact"
}

build_server_bundle() {
  local goos="$1"
  local goarch="$2"
  local root_dir="$OUT_DIR/server/$goos/$goarch"
  mkdir -p "$root_dir"

  build_one "stdiscosrv" "$goos" "$goarch" "$root_dir"
  build_one "strelaysrv" "$goos" "$goarch" "$root_dir"

  if [[ "$INCLUDE_INFRA" == "true" ]]; then
    build_one "strelaypoolsrv" "$goos" "$goarch" "$root_dir"
    build_one "stupgrades" "$goos" "$goarch" "$root_dir"
    build_one "stcrashreceiver" "$goos" "$goarch" "$root_dir"
    build_one "ursrv" "$goos" "$goarch" "$root_dir"
  fi

  cp deploy/docker-compose.optional-servers.yml "$root_dir/"
  cp docs/deploy-servers.md "$root_dir/README-SERVER.md"
  cp docs/ci-cd.md "$root_dir/README-SERVER-CI.md"
}

build_client_bundle() {
  local goos="$1"
  local goarch="$2"
  local out_dir="$OUT_DIR/client/$goos/$goarch"
  mkdir -p "$out_dir"

  build_one "syncthing" "$goos" "$goarch" "$out_dir"
  cp docs/ci-cd.md "$out_dir/README-CLIENT-CI.md"
}

for goos in "${GOOS_ARR[@]}"; do
  for goarch in "${GOARCH_ARR[@]}"; do
    if [[ "$MODE" == "client" || "$MODE" == "both" ]]; then
      build_client_bundle "$goos" "$goarch"
    fi
    if [[ "$MODE" == "server" || "$MODE" == "both" ]]; then
      build_server_bundle "$goos" "$goarch"
    fi
  done
done

echo "Build output:"
echo " - Client brand: $OUT_DIR/client/*/*"
echo " - Server brand: $OUT_DIR/server/*/*"
