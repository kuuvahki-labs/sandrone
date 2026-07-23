#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:-${repo_root}/internal/service/catalog_builtin}"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/sandrone-ruleset-catalog.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT

export GIT_TERMINAL_PROMPT=0

clone_tree_paths() {
  local repository="$1"
  local branch="$2"
  local name="$3"
  local clone_dir="$work_dir/$name"

  git clone \
    --filter=blob:none \
    --no-checkout \
    --no-tags \
    --depth=1 \
    --single-branch \
    --branch "$branch" \
    "$repository" \
    "$clone_dir"
  git -C "$clone_dir" ls-tree -r --name-only HEAD >"$work_dir/$name.paths"
}

checkout_repository_branch() {
  local repository="$1"
  local branch="$2"
  local name="$3"
  local checkout_dir="$work_dir/$name"

  git clone \
    --filter=blob:none \
    --no-checkout \
    --no-tags \
    --depth=1 \
    --single-branch \
    --branch "$branch" \
    "$repository" \
    "$checkout_dir"
  git -C "$checkout_dir" sparse-checkout init --cone
  git -C "$checkout_dir" sparse-checkout set rule/Shadowrocket
  git -C "$checkout_dir" checkout --quiet
}

clone_tree_paths \
  https://github.com/MetaCubeX/meta-rules-dat.git \
  meta \
  meta-rules-dat-meta
clone_tree_paths \
  https://github.com/MetaCubeX/meta-rules-dat.git \
  sing \
  meta-rules-dat-sing
checkout_repository_branch \
  https://github.com/blackmatrix7/ios_rule_script.git \
  master \
  ios-rule-script

"${GO:-go}" run "$repo_root/internal/tools/ruleset-catalog-gen" \
  -output "$work_dir/catalog.json.gz" \
  -metacubex-meta-paths "$work_dir/meta-rules-dat-meta.paths" \
  -metacubex-sing-paths "$work_dir/meta-rules-dat-sing.paths" \
  -shadowrocket-root "$work_dir/ios-rule-script"

gzip -t "$work_dir/catalog.json.gz"

mkdir -p "$output_dir"
rm -f "$output_dir/catalog.json"
install -m 0644 "$work_dir/catalog.json.gz" "$output_dir/catalog.json.gz"
