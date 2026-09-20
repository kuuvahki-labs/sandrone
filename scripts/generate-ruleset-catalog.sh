#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
default_output_dir="$repo_root/internal/service/catalog_builtin"
default_sources_file="$repo_root/internal/tools/ruleset-catalog-gen/sources.lock"

github_mirror="${RULESET_CATALOG_GITHUB_MIRROR:-https://github.com}"
github_mirror="${github_mirror%/}"
sources_file="${RULESET_CATALOG_SOURCES_FILE:-$default_sources_file}"
if [[ "${RULESET_CATALOG_CACHE_DIR+x}" == x ]]; then
  cache_dir="$RULESET_CATALOG_CACHE_DIR"
else
  cache_dir="${XDG_CACHE_HOME:-${HOME:?HOME is required when XDG_CACHE_HOME is unset}/.cache}/sandrone/ruleset-catalog"
fi

meta_repository="$github_mirror/MetaCubeX/meta-rules-dat.git"
shadowrocket_repository="$github_mirror/blackmatrix7/ios_rule_script.git"

export GIT_TERMINAL_PROMPT=0

usage() {
  printf '%s\n' 'usage: generate-ruleset-catalog.sh {generate [output-dir]|update-sources [sources-file]}' >&2
}

validate_revision() {
  local revision="$1"
  [[ "$revision" =~ ^[0-9a-f]{40}$ ]]
}

load_sources() {
  local filename="$1"
  local line_number=0
  local entry key revision

  metacubex_meta_revision=""
  metacubex_sing_revision=""
  shadowrocket_revision=""

  if [[ ! -f "$filename" ]]; then
    printf 'rule-set source lock does not exist: %s\n' "$filename" >&2
    return 1
  fi

  while IFS= read -r entry || [[ -n "$entry" ]]; do
    line_number=$((line_number + 1))
    if [[ -z "$entry" || "$entry" == \#* ]]; then
      continue
    fi
    if [[ "$entry" != *=* ]]; then
      printf '%s:%d: expected KEY=REVISION\n' "$filename" "$line_number" >&2
      return 1
    fi
    key="${entry%%=*}"
    revision="${entry#*=}"
    if ! validate_revision "$revision"; then
      printf '%s:%d: revision for %s must be a lowercase 40-character Git object ID\n' \
        "$filename" "$line_number" "$key" >&2
      return 1
    fi
    case "$key" in
      METACUBEX_META_REVISION)
        if [[ -n "$metacubex_meta_revision" ]]; then
          printf '%s:%d: duplicate %s\n' "$filename" "$line_number" "$key" >&2
          return 1
        fi
        metacubex_meta_revision="$revision"
        ;;
      METACUBEX_SING_REVISION)
        if [[ -n "$metacubex_sing_revision" ]]; then
          printf '%s:%d: duplicate %s\n' "$filename" "$line_number" "$key" >&2
          return 1
        fi
        metacubex_sing_revision="$revision"
        ;;
      SHADOWROCKET_REVISION)
        if [[ -n "$shadowrocket_revision" ]]; then
          printf '%s:%d: duplicate %s\n' "$filename" "$line_number" "$key" >&2
          return 1
        fi
        shadowrocket_revision="$revision"
        ;;
      *)
        printf '%s:%d: unsupported rule-set source %s\n' "$filename" "$line_number" "$key" >&2
        return 1
        ;;
    esac
  done <"$filename"

  for required in \
    METACUBEX_META_REVISION \
    METACUBEX_SING_REVISION \
    SHADOWROCKET_REVISION; do
    case "$required" in
      METACUBEX_META_REVISION) revision="$metacubex_meta_revision" ;;
      METACUBEX_SING_REVISION) revision="$metacubex_sing_revision" ;;
      SHADOWROCKET_REVISION) revision="$shadowrocket_revision" ;;
    esac
    if [[ -z "$revision" ]]; then
      printf '%s: missing %s\n' "$filename" "$required" >&2
      return 1
    fi
  done
}

resolve_revision() {
  local repository="$1"
  local reference="$2"
  local result revision resolved_reference extra

  result="$(git ls-remote --exit-code "$repository" "$reference")"
  read -r revision resolved_reference extra <<<"$result"
  if ! validate_revision "$revision" || [[ "$resolved_reference" != "$reference" || -n "${extra:-}" ]]; then
    printf 'unexpected ls-remote result for %s %s: %s\n' "$repository" "$reference" "$result" >&2
    return 1
  fi
  printf '%s\n' "$revision"
}

update_sources() {
  local filename="$1"
  local directory temporary
  local meta_revision sing_revision shadowrocket_master_revision

  meta_revision="$(resolve_revision "$meta_repository" refs/heads/meta)"
  sing_revision="$(resolve_revision "$meta_repository" refs/heads/sing)"
  shadowrocket_master_revision="$(resolve_revision "$shadowrocket_repository" refs/heads/master)"

  directory="$(dirname "$filename")"
  mkdir -p "$directory"
  temporary="$(mktemp "$directory/.sources.lock.XXXXXX")"
  if ! {
    {
      printf 'METACUBEX_META_REVISION=%s\n' "$meta_revision"
      printf 'METACUBEX_SING_REVISION=%s\n' "$sing_revision"
      printf 'SHADOWROCKET_REVISION=%s\n' "$shadowrocket_master_revision"
    } >"$temporary"
    chmod 0644 "$temporary"
    mv "$temporary" "$filename"
  }; then
    rm -f "$temporary"
    return 1
  fi
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

  if git --git-dir="$cache_repository" remote get-url origin >/dev/null 2>&1; then
    git --git-dir="$cache_repository" remote set-url origin "$repository"
  else
    git --git-dir="$cache_repository" remote add origin "$repository"
  fi
}

ensure_revision() {
  local repository="$1"
  local revision="$2"
  local cache_repository="$3"

  ensure_repository "$repository" "$cache_repository"
  if ! git --git-dir="$cache_repository" cat-file -e "$revision^{commit}" 2>/dev/null; then
    git --git-dir="$cache_repository" fetch --quiet --no-tags --depth=1 --filter=blob:none origin "$revision"
  fi

  if [[ "$(git --git-dir="$cache_repository" rev-parse "$revision^{commit}")" != "$revision" ]]; then
    printf 'fetched revision does not match lock for %s\n' "$repository" >&2
    return 1
  fi
}

write_tree_paths() {
  local cache_repository="$1"
  local revision="$2"
  local output="$3"

  git --git-dir="$cache_repository" ls-tree -r --name-only "$revision" >"$output"
}

extract_repository_revision() {
  local cache_repository="$1"
  local revision="$2"
  local output_dir="$3"

  mkdir -p "$output_dir"
  git --git-dir="$cache_repository" archive "$revision" rule/Shadowrocket | tar -x -C "$output_dir"
}

install_catalog() {
  local source="$1"
  local output_dir="$2"

  if [[ ! -s "$source" ]]; then
    printf 'rule-set catalog is missing or empty: %s\n' "$source" >&2
    return 1
  fi
  gzip -t "$source"
  mkdir -p "$output_dir"
  rm -f "$output_dir/catalog.json"
  install -m 0644 "$source" "$output_dir/catalog.json.gz"
}

generate_catalog() {
  local output_dir="$1"
  local go_command host_goos host_goarch
  local git_cache_dir meta_cache shadowrocket_cache

  load_sources "$sources_file"

  work_dir="$(mktemp -d "${TMPDIR:-/tmp}/sandrone-ruleset-catalog.XXXXXX")"
  trap 'rm -rf "$work_dir"' EXIT

  git_cache_dir="${cache_dir:-$work_dir/git-cache}"
  meta_cache="$git_cache_dir/metacubex-meta-rules-dat.git"
  shadowrocket_cache="$git_cache_dir/blackmatrix7-ios-rule-script.git"

  ensure_revision "$meta_repository" "$metacubex_meta_revision" "$meta_cache"
  ensure_revision "$meta_repository" "$metacubex_sing_revision" "$meta_cache"
  ensure_revision "$shadowrocket_repository" "$shadowrocket_revision" "$shadowrocket_cache"

  write_tree_paths "$meta_cache" "$metacubex_meta_revision" "$work_dir/meta-rules-dat-meta.paths"
  write_tree_paths "$meta_cache" "$metacubex_sing_revision" "$work_dir/meta-rules-dat-sing.paths"
  extract_repository_revision "$shadowrocket_cache" "$shadowrocket_revision" "$work_dir/ios-rule-script"

  go_command=${GO:-go}
  host_goos=$($go_command env GOHOSTOS)
  host_goarch=$($go_command env GOHOSTARCH)

  GOOS="$host_goos" GOARCH="$host_goarch" "$go_command" run "$repo_root/internal/tools/ruleset-catalog-gen" \
    -output "$work_dir/catalog.json.gz" \
    -metacubex-meta-paths "$work_dir/meta-rules-dat-meta.paths" \
    -metacubex-sing-paths "$work_dir/meta-rules-dat-sing.paths" \
    -shadowrocket-root "$work_dir/ios-rule-script"

  install_catalog "$work_dir/catalog.json.gz" "$output_dir"
}

case "${1:-}" in
  generate)
    if [[ "$#" -gt 2 ]]; then
      usage
      exit 2
    fi
    generate_catalog "${2:-$default_output_dir}"
    ;;
  update-sources)
    if [[ "$#" -gt 2 ]]; then
      usage
      exit 2
    fi
    update_sources "${2:-$sources_file}"
    ;;
  *)
    usage
    exit 2
    ;;
esac
