#!/usr/bin/env bash
# Builds rjdl for every supported platform into dist/, exactly like the
# release workflow does:
#
#   scripts/build.sh           version taken from git describe
#   scripts/build.sh v1.2.0    explicit version
set -euo pipefail

cd "$(dirname "$0")/.."
root=$(pwd)
version=${1:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}
ldflags="-s -w -X github.com/MahdiGraph/radio-javan-downloader/internal/cli.Version=$version"
targets=(windows/amd64 windows/arm64 darwin/arm64 darwin/amd64 linux/amd64 linux/arm64)

rm -rf dist
mkdir dist
for target in "${targets[@]}"; do
	os=${target%/*}
	arch=${target#*/}
	name=rjdl_${os}_${arch}
	exe=rjdl
	[[ $os == windows ]] && exe=rjdl.exe
	work=$(mktemp -d)
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "$ldflags" -o "$work/$exe" ./cmd/rjdl
	if [[ $os == windows ]]; then
		(cd "$work" && zip -q "$root/dist/$name.zip" "$exe")
	else
		tar -C "$work" -czf "dist/$name.tar.gz" "$exe"
	fi
	rm -rf "$work"
	echo "dist/$name ($version)"
done

cd dist
if command -v sha256sum >/dev/null; then
	sha256sum -- *.zip *.tar.gz >checksums.txt
else
	shasum -a 256 -- *.zip *.tar.gz >checksums.txt
fi
