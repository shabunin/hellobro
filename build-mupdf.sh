#!/bin/sh
# Builds MuPDF for the Kobo (32-bit ARM, Cortex-A7), as static libraries
# hellobro's PDF reader links in (package pdf, cgo):
#   third_party/mupdf/include, libmupdf.a, libmupdf-third.a
# They're big and made from a pinned release, so they're not in git: run
# this once (make does when they're missing). Only what reading PDFs takes:
# no JavaScript, HTML/EPUB, XPS, SVG or barcodes; of the fonts, the PDF
# base 14 and Charis SIL (Latin, Cyrillic, Greek), not the CJK, Noto or
# emoji ones. Builds in the hellobro-cross image (tools/Containerfile).
#
# usage: ./build-mupdf.sh [path/to/mupdf-source]
set -eu
ver=1.28.5
sum=98a5c10cda20c3992cdf76ff6b2a1149c32bd79cc796d3f703230b1185b7e934 # mupdf.com/releases
here=$(cd "$(dirname "$0")" && pwd)
src=${1:-$here/../mupdf-$ver-source}
if [ ! -d "$src" ]; then
	tgz=$here/../mupdf-$ver-source.tar.gz
	[ -f "$tgz" ] || curl -sSfL -o "$tgz" "https://mupdf.com/downloads/archive/mupdf-$ver-source.tar.gz"
	echo "$sum  $tgz" | sha256sum -c -
	tar xzf "$tgz" -C "$(dirname "$src")"
fi
src=$(cd "$src" && pwd)
out=$here/third_party/mupdf
mkdir -p "$out"

podman run --rm --security-opt label=disable -v "$src":/src -w /src hellobro-cross sh -euc '
	make -j"$(nproc)" build=release OS=Linux libs \
		CC=arm-linux-gnueabihf-gcc CXX=arm-linux-gnueabihf-g++ AR=arm-linux-gnueabihf-ar LD=arm-linux-gnueabihf-ld \
		HAVE_X11=no HAVE_GLUT=no HAVE_CURL=no HAVE_LIBCRYPTO=no USE_SYSTEM_LIBS=no \
		mujs=no html=no xps=no svg=no extract=no brotli=no barcode=no tesseract=no \
		XCFLAGS="-mcpu=cortex-a7 -mfpu=neon-vfpv4 -mfloat-abi=hard -DTOFU -DTOFU_CJK -DTOFU_EMOJI -DTOFU_HISTORIC -DTOFU_SYMBOL -DFZ_ENABLE_ICC=0 -DFZ_ENABLE_OCR_OUTPUT=0 -DFZ_ENABLE_ODT_OUTPUT=0 -DFZ_ENABLE_CBZ=0"
'
lib=$(ls -d "$src"/build/release*/ | head -1)
rm -rf "$out/include"
cp -r "$src/include" "$out/include"
cp "$lib/libmupdf.a" "$lib/libmupdf-third.a" "$out/"
ls -la "$out"
