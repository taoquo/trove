#!/usr/bin/env bash
# Produces the reference pictures and box trees from the extension itself.
#
#   bash tools/parity/render.sh                 # the eight states at 1297x808
#   bash tools/parity/render.sh detail dark     # just these states
#
# Requires a built extension (npm run build) and Google Chrome. Output lands in
# docs/parity/, which is gitignored with the rest of docs/.
set -euo pipefail
cd "$(dirname "$0")/../.."

STATES=("$@")
if [ ${#STATES[@]} -eq 0 ]; then
  STATES=(workspace detail list settings dark)
fi

for state in "${STATES[@]}"; do
  node tools/parity/render-source.mjs --state "$state" --width 1297 --height 808
done
