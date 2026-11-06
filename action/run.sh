#!/usr/bin/env bash
# Expands the action's path patterns and runs munin lint on the matching files.
set -euo pipefail
shopt -s globstar nullglob

munin="${MUNIN_BIN:-munin}"

includes=()
excludes=()
while IFS= read -r line || [[ -n "$line" ]]; do
    read -ra words <<<"$line"
    for word in "${words[@]}"; do
        if [[ "$word" == !* ]]; then
            excludes+=("${word#!}")
        else
            includes+=("$word")
        fi
    done
done <<<"${INPUT_PATHS:-**/screen.yaml}"

failed=0
files=()
declare -A seen=()
for pattern in "${includes[@]}"; do
    # shellcheck disable=SC2206 # the pattern is meant to be glob-expanded
    matches=($pattern)
    if ((${#matches[@]} == 0)); then
        echo "::error title=munin lint::Pattern '${pattern}' matched no files"
        failed=1
        continue
    fi
    for file in "${matches[@]}"; do
        file="${file#./}"
        [[ -f "$file" ]] || continue
        for exclude in "${excludes[@]}"; do
            # shellcheck disable=SC2053 # the exclude is meant to be a pattern
            [[ "$file" == ${exclude#./} ]] && continue 2
        done
        [[ -n "${seen[$file]:-}" ]] && continue
        seen[$file]=1
        files+=("$file")
    done
done

if ((${#files[@]} == 0)); then
    echo "::error title=munin lint::No screen files to lint"
    exit 1
fi

args=(lint --format github)
if [[ "${INPUT_STRICT:-false}" == "true" ]]; then
    args+=(--strict)
fi

"$munin" "${args[@]}" "${files[@]}" || failed=1
exit "$failed"
