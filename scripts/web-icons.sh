#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
# Renders the PNG app icons of the web client from their SVG sources in app/web/public (ADR-0028):
# the 192 and 512 px icons that Chrome's install criteria ask for, a 512 px maskable icon, and the
# 180 px apple-touch-icon.png that iOS puts on the home screen. The PNGs are committed; run this after
# changing icon.svg or icon-maskable.svg. Needs ImageMagick (convert) with an SVG renderer.
# Usage: scripts/web-icons.sh
set -euo pipefail

public="$(cd "$(dirname "$0")/.." && pwd)/app/web/public"

render() {
	local source="$1" size="$2" out="$3"
	# Render at twice the size, then scale down, for smooth edges; strip metadata so the output is stable.
	convert -background none -density "$((size * 2 * 72 / 512))" "$public/$source" \
		-resize "${size}x${size}" -strip "PNG32:$public/$out"
}

render icon.svg 192 icon-192.png
render icon.svg 512 icon-512.png
render icon-maskable.svg 512 icon-maskable-512.png
render icon-maskable.svg 180 apple-touch-icon.png
