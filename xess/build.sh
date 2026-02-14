#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"
NPX_CMD=(npx --no-install)
"${NPX_CMD[@]}" postcss ./xess.css -o xess.min.css
