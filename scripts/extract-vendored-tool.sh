#!/bin/bash
set -euo pipefail

if [ "$#" -ne 4 ]; then
  echo "Usage: $0 <tool: fd|rg> <target-os: linux|linux-musl|darwin|windows> <arch: amd64|arm64> <dest>" >&2
  exit 1
fi

TOOL="$1"
TARGET_OS="$2"
ARCH="$3"
DEST="$4"

case "$TOOL" in
  fd)
    PKG_DIR="pkgs/fd"
    BIN_NAME="fd"
    ;;
  rg)
    PKG_DIR="pkgs/ripgrep"
    BIN_NAME="rg"
    ;;
  *)
    echo "Unsupported tool: $TOOL" >&2
    exit 1
    ;;
esac

if [ ! -d "$PKG_DIR" ]; then
  echo "Vendored package directory not found: $PKG_DIR" >&2
  exit 1
fi

VERSION_DIR=$(cat "$PKG_DIR/LATEST")
MANIFEST="$PKG_DIR/$VERSION_DIR/manifest.tsv"
if [ ! -f "$MANIFEST" ]; then
  echo "Manifest not found: $MANIFEST" >&2
  exit 1
fi

TARGET="${TARGET_OS}/${ARCH}"
FILENAME=$(awk -F $'\t' -v target="$TARGET" 'NR > 1 && $1 == target { print $2; exit }' "$MANIFEST")
if [ -z "$FILENAME" ] || [ "$FILENAME" = "MISSING" ]; then
  echo "Missing vendored $TOOL binary for $TARGET in $MANIFEST" >&2
  exit 1
fi

ARCHIVE="$PKG_DIR/$VERSION_DIR/$FILENAME"
if [ ! -f "$ARCHIVE" ]; then
  echo "Vendored archive not found: $ARCHIVE" >&2
  exit 1
fi

mkdir -p "$(dirname "$DEST")"
rm -f "$DEST"

extract_from_tar() {
  tar -xOf "$ARCHIVE" --wildcards "*/$1" > "$DEST"
}

extract_from_zip() {
  unzip -p "$ARCHIVE" "*/$1" > "$DEST"
}

extract_from_deb() {
  dpkg-deb --fsys-tarfile "$ARCHIVE" | tar -xOf - "./usr/bin/$1" > "$DEST"
}

case "$FILENAME" in
  *.tar.gz)
    extract_from_tar "$BIN_NAME"
    ;;
  *.zip)
    extract_from_zip "${BIN_NAME}.exe"
    ;;
  *.deb)
    extract_from_deb "$BIN_NAME"
    ;;
  *)
    echo "Unsupported vendored archive format: $ARCHIVE" >&2
    exit 1
    ;;
esac

chmod +x "$DEST" 2>/dev/null || true

echo "Extracted $TOOL for $TARGET -> $DEST"
