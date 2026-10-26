#!/bin/sh
# Builds steth for every release target into dist/, one archive each, with
# their SHA-256 sums, and prints the CHANGELOG.md section for the version.
# usage: scripts/release.sh v0.1.0
set -eu

version=${1:?usage: scripts/release.sh VERSION, such as v0.1.0}
targets="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"

notes=$(awk -v v="## $version" '$0 == v {on = 1; next} /^## / {on = 0} on' CHANGELOG.md)
if [ -z "$notes" ]; then
	echo "release: CHANGELOG.md has no \"## $version\" section; add one first" >&2
	exit 1
fi

rm -rf dist
mkdir -p dist
for target in $targets; do
	os=${target%/*}
	arch=${target#*/}
	name=steth-$version-$os-$arch
	binary=steth
	[ "$os" = windows ] && binary=steth.exe
	mkdir -p "dist/$name"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
		-ldflags "-s -w -X main.version=$version" -o "dist/$name/$binary" ./cmd/steth
	cp LICENSE README.md "dist/$name/"
	if [ "$os" = windows ]; then
		(cd dist && zip -qr "$name.zip" "$name")
	else
		tar -C dist -czf "dist/$name.tar.gz" "$name"
	fi
	rm -r "dist/$name"
done
(cd dist && sha256sum ./*.tar.gz ./*.zip | sed 's| \./| |' > SHA256SUMS)
printf '%s\n' "$notes" > dist/NOTES.md
