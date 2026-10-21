#!/bin/sh
# Builds FBInk for Kobo with image + TrueType support:
#   - third_party/fbink/libfbink.a + fbink.h, linked into hellobro (cgo);
#   - bin/fbink, a static CLI (fallback, and handy for poking at the screen).
# KOReader ships fbink without image support, and hellobro blits its UI as
# images, so we bring our own. Builds in the hellobro-cross image (tools/Containerfile).
#
# usage: ./build-fbink.sh [path/to/FBInk-checkout]
set -eu
here=$(cd "$(dirname "$0")" && pwd)
src=$(cd "${1:-$here/../fbink-src}" && pwd)
mkdir -p "$here/bin" "$here/third_party/fbink"

podman run --rm --security-opt label=disable -v "$src":/src -w /src hellobro-cross sh -euc '
	git config --global --add safe.directory "*"
	make clean >/dev/null 2>&1 || true
	make static KOBO=true MINIMAL=true DRAW=true BITMAP=true IMAGE=true OPENTYPE=true \
		CROSS_TC=arm-linux-gnueabihf LDFLAGS=-static
	arm-linux-gnueabihf-strip Release/fbink
'
cp "$src/Release/fbink" "$here/bin/fbink"
cp "$src/Release/libfbink.a" "$src/fbink.h" "$here/third_party/fbink/"
file "$here/bin/fbink"
