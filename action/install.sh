#!/usr/bin/env bash
# Downloads the munin release binary for the GitHub Action and puts it on PATH.
set -euo pipefail

# Actions are checked out to .../_actions/<owner>/<repo>/<ref>
repo="naueramant/munin"
ref=""
if [[ "${GITHUB_ACTION_PATH:-}" == */_actions/*/*/* ]]; then
    rest="${GITHUB_ACTION_PATH#*/_actions/}"
    repo="${rest%/*}"
    ref="${rest##*/}"
fi

version="${INPUT_VERSION:-}"
if [[ -z "$version" ]]; then
    if [[ "$ref" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        version="$ref"
    else
        version="latest"
    fi
fi
if [[ "$version" == "latest" ]]; then
    version="$(gh release view --repo "$repo" --json tagName --jq .tagName)"
fi

if [[ "$(uname -s)" != "Linux" ]]; then
    echo "::error title=munin lint::Only Linux runners are supported"
    exit 1
fi
case "$(uname -m)" in
    x86_64 | amd64) arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *)
        echo "::error title=munin lint::Unsupported runner architecture $(uname -m)"
        exit 1
        ;;
esac

asset="munin_${version#v}_linux_${arch}.tar.gz"
checksums="munin_${version#v}_checksums.txt"
dir="${RUNNER_TEMP:-$(mktemp -d)}/munin-${version}"
mkdir -p "$dir"

echo "Downloading ${asset} from ${repo} release ${version}"
gh release download "$version" --repo "$repo" --pattern "$asset" --pattern "$checksums" --dir "$dir" --clobber
(cd "$dir" && grep " ${asset}\$" "$checksums" | sha256sum --check --strict -)
tar -xzf "$dir/$asset" -C "$dir" munin

echo "$dir" >>"$GITHUB_PATH"
"$dir/munin" --version
