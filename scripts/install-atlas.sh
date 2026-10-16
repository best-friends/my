#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
# Installs the pinned Atlas Community Edition binary (Apache-2.0, ADR-0023) and verifies its checksum.
# Usage: scripts/install-atlas.sh [DEST]   (default: bin/atlas)
# To upgrade, change ATLAS_VERSION and every checksum below; the values come from
# https://release.ariga.io/atlas/atlas-community-<os>-<arch>-v<version>.sha256
set -euo pipefail

ATLAS_VERSION=1.3.0

root="$(cd "$(dirname "$0")/.." && pwd)"
dest="${1:-$root/bin/atlas}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*)
	echo "install-atlas: unsupported architecture: $arch" >&2
	exit 1
	;;
esac

case "$os-$arch" in
linux-amd64) sum=10d7913e3dce43ab99b8d71534a4cbadaf11a16dc293adf3b91d10e83a0ac70b ;;
linux-arm64) sum=082188c57a53439596a3ee52173b12e785aa83ec718505674e2052db0fec0c4c ;;
darwin-amd64) sum=650981a024301775ec964e5134e2d5712b7ef1b25fec4b2ec54bad762b4bdf6f ;;
darwin-arm64) sum=4e5ffdc10b2b4fd3a06074aba72848150907d5316cacad7b19bae6c6ae3db991 ;;
*)
	echo "install-atlas: unsupported platform: $os-$arch" >&2
	exit 1
	;;
esac

if [[ -x "$dest" ]] && "$dest" version 2>/dev/null | grep -q "v$ATLAS_VERSION\b"; then
	exit 0
fi

url="https://release.ariga.io/atlas/atlas-community-$os-$arch-v$ATLAS_VERSION"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

echo "install-atlas: downloading $url" >&2
curl -sSfL --retry 3 -o "$tmp" "$url"

if command -v sha256sum >/dev/null; then
	got="$(sha256sum "$tmp" | awk '{ print $1 }')"
else
	got="$(shasum -a 256 "$tmp" | awk '{ print $1 }')"
fi
if [[ "$got" != "$sum" ]]; then
	echo "install-atlas: checksum mismatch for $url: want $sum, got $got" >&2
	exit 1
fi

mkdir -p "$(dirname "$dest")"
chmod +x "$tmp"
mv "$tmp" "$dest"
trap - EXIT
"$dest" version
