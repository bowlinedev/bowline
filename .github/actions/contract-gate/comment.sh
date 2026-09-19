#!/usr/bin/env bash
set -euo pipefail

report="$1"
marker="<!-- bowline-contract-gate:${DIRECTORY} -->"
number="$(jq -r '.pull_request.number // empty' "$GITHUB_EVENT_PATH")"
if [ -z "$number" ]; then
  exit 0
fi
body="$(printf '%s\n### Contract changes in `%s`\n\n%s\n' "$marker" "$DIRECTORY" "$(cat "$report")")"
fork="$(jq -r '.pull_request.head.repo.full_name // empty' "$GITHUB_EVENT_PATH")"
if [ -n "$fork" ] && [ "$fork" != "$GITHUB_REPOSITORY" ]; then
  echo "contract gate: pull request comes from $fork, so the token cannot comment; the report is in the step log above" >&2
  exit 0
fi

existing="$(gh api "repos/${GITHUB_REPOSITORY}/issues/${number}/comments" --paginate --jq ".[] | select(.body | startswith(\"$marker\")) | .id" | head -n1 || true)"
if [ -n "$existing" ]; then
  target=(-X PATCH "repos/${GITHUB_REPOSITORY}/issues/comments/${existing}")
else
  target=(-X POST "repos/${GITHUB_REPOSITORY}/issues/${number}/comments")
fi
if ! gh api "${target[@]}" -f body="$body" >/dev/null; then
  echo "contract gate: could not post the report as a comment; it is in the step log above" >&2
fi
