#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
default_output_dir="$repo_root/internal/service/catalog_builtin"
default_sources_file="$repo_root/internal/tools/ruleset-catalog-gen/sources.lock"
output_dir="${1:-$default_output_dir}"
sources_file="${RULESET_CATALOG_SOURCES_FILE:-$default_sources_file}"

if [[ "$#" -gt 1 ]]; then
  printf '%s\n' 'usage: generate-ruleset-catalog.sh [output-dir]' >&2
  exit 2
fi

github_mirror="${RULESET_CATALOG_GITHUB_MIRROR:-https://github.com}"
github_mirror="${github_mirror%/}"
if [[ "${RULESET_CATALOG_CACHE_DIR+x}" == x ]]; then
  cache_dir="$RULESET_CATALOG_CACHE_DIR"
else
  cache_dir="${XDG_CACHE_HOME:-${HOME:?HOME is required when XDG_CACHE_HOME is unset}/.cache}/sandrone/ruleset-catalog"
fi

meta_repository="$github_mirror/MetaCubeX/meta-rules-dat.git"
shadowrocket_repository="$github_mirror/blackmatrix7/ios_rule_script.git"

export GIT_TERMINAL_PROMPT=0

validate_revision() {
  local revision="$1"
  [[ "$revision" =~ ^[0-9a-f]{40}$ ]]
}

load_sources() {
  local filename="$1"
  local name revision

  METACUBEX_META_REVISION=""
  METACUBEX_SING_REVISION=""
  SHADOWROCKET_REVISION=""

  if [[ ! -f "$filename" ]]; then
    printf 'rule-set source lock does not exist: %s\n' "$filename" >&2
    return 1
  fi

  # The lock is tracked next to the generator and contains only assignments.
  # shellcheck disable=SC1090
  source "$filename"
  for name in METACUBEX_META_REVISION METACUBEX_SING_REVISION SHADOWROCKET_REVISION; do
    revision="${!name}"
    if ! validate_revision "$revision"; then
      printf '%s: %s must be a lowercase 40-character Git object ID\n' \
        "$filename" "$name" >&2
      return 1
    fi
  done

  metacubex_meta_revision="$METACUBEX_META_REVISION"
  metacubex_sing_revision="$METACUBEX_SING_REVISION"
  shadowrocket_revision="$SHADOWROCKET_REVISION"
}

ensure_repository() {
  local repository="$1"
  local cache_repository="$2"

  if [[ ! -e "$cache_repository" ]]; then
    mkdir -p "$(dirname "$cache_repository")"
    git init --bare --quiet "$cache_repository"
  elif [[ "$(git --git-dir="$cache_repository" rev-parse --is-bare-repository 2>/dev/null)" != true ]]; then
    printf 'rule-set Git cache is not a bare repository: %s\n' "$cache_repository" >&2
    return 1
  fi

  git --git-dir="$cache_repository" config remote.origin.url "$repository"
}

ensure_revision() {
  local repository="$1"
  local revision="$2"
  local cache_repository="$3"

  ensure_repository "$repository" "$cache_repository"
  if ! git --git-dir="$cache_repository" cat-file -e "$revision^{commit}" 2>/dev/null; then
    git --git-dir="$cache_repository" fetch --quiet --no-tags --depth=1 --filter=blob:none origin "$revision"
  fi

  if ! git --git-dir="$cache_repository" cat-file -e "$revision^{commit}" 2>/dev/null; then
    printf 'rule-set revision is unavailable after fetch: %s\n' "$revision" >&2
    return 1
  fi
}

load_sources "$sources_file"

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/sandrone-ruleset-catalog.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT

git_cache_dir="${cache_dir:-$work_dir/git-cache}"
meta_cache="$git_cache_dir/metacubex-meta-rules-dat.git"
shadowrocket_cache="$git_cache_dir/blackmatrix7-ios-rule-script.git"

ensure_revision "$meta_repository" "$metacubex_meta_revision" "$meta_cache"
ensure_revision "$meta_repository" "$metacubex_sing_revision" "$meta_cache"
ensure_revision "$shadowrocket_repository" "$shadowrocket_revision" "$shadowrocket_cache"

git --git-dir="$meta_cache" ls-tree -r --name-only "$metacubex_meta_revision" >"$work_dir/meta-rules-dat-meta.paths"
git --git-dir="$meta_cache" ls-tree -r --name-only "$metacubex_sing_revision" >"$work_dir/meta-rules-dat-sing.paths"
mkdir -p "$work_dir/ios-rule-script"
git --git-dir="$shadowrocket_cache" archive "$shadowrocket_revision" rule/Shadowrocket | \
  tar -x -C "$work_dir/ios-rule-script"

go_command=${GO:-go}
host_goos=$($go_command env GOHOSTOS)
host_goarch=$($go_command env GOHOSTARCH)

GOOS="$host_goos" GOARCH="$host_goarch" "$go_command" run "$repo_root/internal/tools/ruleset-catalog-gen" \
  -output "$work_dir/catalog.json.gz" \
  -metacubex-meta-paths "$work_dir/meta-rules-dat-meta.paths" \
  -metacubex-sing-paths "$work_dir/meta-rules-dat-sing.paths" \
  -shadowrocket-root "$work_dir/ios-rule-script"

gzip -t "$work_dir/catalog.json.gz"
mkdir -p "$output_dir"
rm -f "$output_dir/catalog.json"
install -m 0644 "$work_dir/catalog.json.gz" "$output_dir/catalog.json.gz"
