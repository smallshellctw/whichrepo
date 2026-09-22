#!/bin/sh
set -eu

repo="smallshellctw/whichrepo"
version="${WHICHREPO_VERSION:-latest}"
install_dir="${INSTALL_DIR:-$HOME/.local/bin}"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in darwin|linux) ;; *) echo "Unsupported OS: $os" >&2; exit 1 ;; esac
case "$arch" in x86_64|amd64) arch="amd64" ;; arm64|aarch64) arch="arm64" ;; *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; esac

if [ "$version" = "latest" ]; then
  version="$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')"
fi
archive="whichrepo_${version#v}_${os}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$version"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$base/$archive" -o "$tmp/$archive"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"
expected="$(grep " $archive$" "$tmp/checksums.txt" | awk '{print $1}')"
if command -v shasum >/dev/null 2>&1; then actual="$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')"; else actual="$(sha256sum "$tmp/$archive" | awk '{print $1}')"; fi
[ "$expected" = "$actual" ] || { echo "Checksum verification failed" >&2; exit 1; }
tar -xzf "$tmp/$archive" -C "$tmp"
mkdir -p "$install_dir"
install -m 0755 "$tmp/whichrepo" "$install_dir/whichrepo"
echo "Installed whichrepo to $install_dir/whichrepo"
