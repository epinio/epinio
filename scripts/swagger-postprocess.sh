#!/usr/bin/env bash
# Post-process the generated OpenAPI spec in place:
#  - order each path's operations GET, POST, PUT, PATCH, DELETE. go-swagger
#    emits a fixed library order (DELETE before PATCH), and Redoc, which
#    renders the docs site, keeps the spec's order.
#  - add the tag list with descriptions and display names, which
#    swagger:meta cannot express.
set -euo pipefail

spec="${1:-docs/references/api/swagger.json}"
tags="$(dirname "$0")/../docs/references/api/tags.json"

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

jq --slurpfile tags "$tags" '
  def rank: {"parameters": 0, "get": 1, "post": 2, "put": 3,
             "patch": 4, "delete": 5, "options": 6, "head": 7}[.] // 8;
  .paths |= with_entries(
    .value |= (to_entries | sort_by(.key | rank) | from_entries)
  )
  | .tags = $tags[0]
' "$spec" > "$tmp"

mv "$tmp" "$spec"
