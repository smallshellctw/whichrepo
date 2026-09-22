#!/bin/sh
set -eu

binary="${1:-./bin/whichrepo}"
case "$binary" in
  /*) ;;
  *) binary="$(pwd)/$binary" ;;
esac

test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT
mkdir -p "$test_root/bin" "$test_root/home" "$test_root/workspace/payment-api"
cp "$binary" "$test_root/bin/whichrepo"
chmod 0755 "$test_root/bin/whichrepo"
printf '%s\n' '{"name":"payment-api","version":"0.1.0"}' > "$test_root/workspace/payment-api/package.json"

env -i \
  HOME="$test_root/home" \
  PATH="$test_root/bin:/usr/bin:/bin" \
  "$test_root/bin/whichrepo" --help >/dev/null

env -i \
  HOME="$test_root/home" \
  PATH="$test_root/bin:/usr/bin:/bin" \
  WHICHREPO_DB="$test_root/index.db" \
  "$test_root/bin/whichrepo" init "$test_root/workspace" >/dev/null

env -i \
  HOME="$test_root/home" \
  PATH="$test_root/bin:/usr/bin:/bin" \
  WHICHREPO_DB="$test_root/index.db" \
  "$test_root/bin/whichrepo" route --workspace "$test_root/workspace" "change payment API" >/dev/null

doctor_output="$(env -i \
  HOME="$test_root/home" \
  PATH="$test_root/bin:/usr/bin:/bin" \
  WHICHREPO_DB="$test_root/index.db" \
  "$test_root/bin/whichrepo" doctor --workspace "$test_root/workspace")"
printf '%s' "$doctor_output" | grep '"cgo_required": false' >/dev/null
printf '%s' "$doctor_output" | grep '"runtime_dependencies": \[\]' >/dev/null

echo "Standalone binary smoke test passed (no Go or language runtime on PATH)."
