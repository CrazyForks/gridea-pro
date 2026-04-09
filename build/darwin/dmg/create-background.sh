#!/usr/bin/env bash
# =============================================================================
# Gridea Pro — macOS DMG installer background generator
# -----------------------------------------------------------------------------
# 把 background.svg 渲染成一张 hidpi background.tiff（包含 1x + 2x 两份位图），
# create-dmg 用这张 tiff 作为窗口背景，retina 屏幕上像素级清晰、无缩放损失。
#
# 用法：
#   ./create-background.sh [output.tiff]
#
# 依赖（macos-latest GitHub runner 全部已预装或可 brew 安装）：
#   - librsvg     (rsvg-convert)   ← 把 SVG 渲染成 PNG，质量远好于 ImageMagick
#   - tiffutil    (macOS 自带)     ← 把 1x + 2x PNG 合成 hidpi tiff
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SVG="${SCRIPT_DIR}/background.svg"
OUT="${1:-background.tiff}"

if ! command -v rsvg-convert >/dev/null 2>&1; then
  echo "✗ rsvg-convert 未安装。请先 brew install librsvg" >&2
  exit 1
fi
if ! command -v tiffutil >/dev/null 2>&1; then
  echo "✗ tiffutil 未找到（应为 macOS 自带工具）" >&2
  exit 1
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# 1x = 600x400  → 普通屏幕
rsvg-convert -w 600  -h 400  "$SVG" -o "${TMP}/bg-1x.png"
# 2x = 1200x800 → retina 屏幕
rsvg-convert -w 1200 -h 800  "$SVG" -o "${TMP}/bg-2x.png"

# 合成 hidpi tiff：Finder 会根据屏幕分辨率自动选用对应像素
tiffutil -cathidpicheck "${TMP}/bg-1x.png" "${TMP}/bg-2x.png" -out "$OUT" >/dev/null

echo "✔ Generated DMG background → $OUT"
