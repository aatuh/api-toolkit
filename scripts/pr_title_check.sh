#!/usr/bin/env bash
set -euo pipefail

title=${PR_TITLE-}

if [[ -z "$title" ]]; then
  echo "pull-request title is required" >&2
  exit 2
fi
if [[ "$title" =~ [[:cntrl:]] ]]; then
  echo "pull-request title must not contain control characters" >&2
  exit 2
fi
if ((${#title} > 256)); then
  echo "pull-request title must be at most 256 characters" >&2
  exit 2
fi

conventional_title='^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([a-z0-9][a-z0-9._/-]*\))?!?: [^[:space:]].*$'
if [[ ! "$title" =~ $conventional_title ]]; then
  echo "pull-request title must use conventional commit syntax, for example: feat(scope): summary" >&2
  exit 2
fi

echo "pull-request title is valid"
