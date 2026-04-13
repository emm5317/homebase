#!/bin/bash
# Download Preact, hooks, and HTM for local vendoring (offline Pi use)
set -euo pipefail

VENDOR_DIR="$(dirname "$0")/../static/vendor"
mkdir -p "$VENDOR_DIR"

echo "Downloading Preact..."
curl -sL "https://esm.sh/preact@10.25.4?bundle" -o "$VENDOR_DIR/preact.module.js"

echo "Downloading Preact hooks..."
curl -sL "https://esm.sh/preact@10.25.4/hooks?bundle" -o "$VENDOR_DIR/hooks.module.js"

echo "Downloading HTM..."
curl -sL "https://esm.sh/htm@3.1.1?bundle" -o "$VENDOR_DIR/htm.module.js"

echo "Done. Update static/index.html imports to use /vendor/ paths."
echo "  import { h, render } from '/vendor/preact.module.js';"
echo "  import { useState, ... } from '/vendor/hooks.module.js';"
echo "  import htm from '/vendor/htm.module.js';"
