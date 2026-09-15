#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
status=0
for guide in "$repo"/docs/guides/*.md; do
  while IFS= read -r line; do
    file="${line#*rom \`}"
    file="${file%%\`*}"
    if [ ! -f "$repo/$file" ]; then
      echo "check-snippets: $(basename "$guide") names missing file $file" >&2
      status=1
      continue
    fi
    block="$(awk -v needle="$line" '
      $0 == needle { found = 1; next }
      found && /^```/ { if (inside) { exit } inside = 1; next }
      inside { print }
    ' "$guide")"
    if [ -z "$block" ]; then
      echo "check-snippets: $(basename "$guide"): no fenced block after \"$line\"" >&2
      status=1
      continue
    fi
    while IFS= read -r snippet; do
      trimmed="$(printf '%s' "$snippet" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')"
      [ -n "$trimmed" ] || continue
      if ! grep -qF -- "$trimmed" "$repo/$file"; then
        echo "check-snippets: $(basename "$guide"): line not found in $file: $trimmed" >&2
        status=1
      fi
    done <<< "$block"
  done < <(grep -E '(^|[^A-Za-z])[Ff]rom `[^`]+`:' "$guide" || true)
done
exit $status
