#!/bin/sh
# Regenerates the TUI captures from the TUI's own rendering: the landing
# page's board (website/src/assets/tui-board.html) and the docs' screenshots
# (docs/images/tui-*.png). Run it when the look of the TUI changes.
#
# Needs Go and Google Chrome (set CHROME to its executable if it isn't in the
# macOS default place). The screenshots use Menlo, as the site does for TUI
# captures, so run it on macOS for the same look.
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
chrome=${CHROME:-"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

(cd "$root" && go test -count=1 -run TestCaptures ./internal/tui -capture "$tmp")
cp "$tmp/tui-board.html" "$root/website/src/assets/tui-board.html"

mkdir -p "$root/docs/images"
for f in "$tmp"/tui-*.html; do
	name=$(basename "$f" .html)
	# The first line is a comment; the rest are the screen's rows.
	rows=$(wc -l <"$f")
	{
		printf '<!doctype html><meta charset="utf-8"><style>'
		printf 'html,body{margin:0;background:#172022}'
		printf 'pre{margin:0;padding:20px 24px;color:#e6eee9;font:14px/1.16 Menlo,monospace}'
		printf '.ch{display:inline-block;width:1ch;text-align:center}'
		printf '</style><pre>'
		tail -n +2 "$f"
		printf '</pre>'
	} >"$tmp/$name.page.html"
	# 140 columns of 14px Menlo are about 1180px wide, and each row is 16.24px;
	# at that line height its box-drawing characters join up.
	"$chrome" --headless=new --disable-gpu --hide-scrollbars --force-device-scale-factor=2 \
		--window-size=1228,$((rows * 1624 / 100 + 40)) \
		--screenshot="$root/docs/images/$name.png" "file://$tmp/$name.page.html" 2>/dev/null
	echo "Wrote docs/images/$name.png"
done
