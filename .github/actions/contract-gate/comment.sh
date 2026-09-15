#!/usr/bin/env bash
set -euo pipefail

report="$1"
marker="<!-- bowline-contract-gate:${DIRECTORY} -->"
number="$(jq -r '.pull_request.number // empty' "$GITHUB_EVENT_PATH")"
if [ -z "$number" ]; then
  exit 0
fi
body="$(printf '%s\n### Contract changes in `%s`\n\n%s\n' "$marker" "$DIRECTORY" "$(cat "$report")")"
existing="$(gh api "repos/${GITHUB_REPOSITORY}/issues/${number}/comments" --paginate --jq ".[] | select(.body | startswith(\"$marker\")) | .id" | head -n1)"
if [ -n "$existing" ]; then
  gh api -X PATCH "repos/${GITHUB_REPOSITORY}/issues/comments/${existing}" -f body="$body" >/dev/null
else
  gh api -X POST "repos/${GITHUB_REPOSITORY}/issues/${number}/comments" -f body="$body" >/dev/null
fi
