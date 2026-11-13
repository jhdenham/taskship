#!/bin/sh
# Install Taskship:
#
#   curl -fsSL https://taskship.dev/install.sh | sh
#
# Downloads the taskship binary for this system from the latest GitHub
# release, checks it against the release's checksums, and installs it into
# ~/.local/bin. Nothing else is written: Taskship creates ~/.taskship on
# first use.
#
#   TASKSHIP_INSTALL_DIR  install into this folder instead of ~/.local/bin
#   TASKSHIP_VERSION      install this release (such as v0.1.0), not the latest
#
# Release assets this script expects: taskship_<os>_<arch>.tar.gz for os in
# darwin, linux and arch in amd64, arm64, each containing the taskship binary
# at the top level, plus checksums.txt in `sha256sum` format.
set -eu

repo=jhdenham/taskship
dir=${TASKSHIP_INSTALL_DIR:-$HOME/.local/bin}
version=${TASKSHIP_VERSION:-latest}

fail() {
  printf 'taskship install: %s\n' "$1" >&2
  exit 1
}

case $(uname -s) in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "Taskship runs on macOS and Linux, not $(uname -s)" ;;
esac
case $(uname -m) in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "no Taskship build for $(uname -m) processors" ;;
esac

if [ "$version" = latest ]; then
  base=https://github.com/$repo/releases/latest/download
else
  base=https://github.com/$repo/releases/download/$version
fi
archive=taskship_${os}_${arch}.tar.gz

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $archive"
curl -fsSL -o "$tmp/$archive" "$base/$archive" || fail "couldn't download $base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || fail "couldn't download $base/checksums.txt"

want=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || fail "checksums.txt has no entry for $archive"
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$archive" | awk '{ print $1 }')
else
  got=$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')
fi
[ "$got" = "$want" ] || fail "checksum mismatch for $archive; nothing was installed"

tar -xzf "$tmp/$archive" -C "$tmp" taskship
mkdir -p "$dir"
cp "$tmp/taskship" "$dir/taskship.new"
chmod 755 "$dir/taskship.new"
mv "$dir/taskship.new" "$dir/taskship"

echo "✓ Installed $("$dir/taskship" --version) in $dir"
case :$PATH: in
  *":$dir:"*) ;;
  *)
    echo
    echo "$dir isn't on your PATH. Add this line to your shell's startup file:"
    echo "  export PATH=\"$dir:\$PATH\""
    ;;
esac
