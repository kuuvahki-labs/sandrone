#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:-${repo_root}/internal/service/catalog_builtin}"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/sandrone-ruleset-catalog.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT

export GIT_TERMINAL_PROMPT=0

blackmatrix_commit="e69663d642551aa3e0164a656179335a896127ad"

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

checkout_repository_at_commit() {
  local repository="$1"
  local commit="$2"
  local name="$3"
  local checkout_dir="$work_dir/$name"

  git init --quiet "$checkout_dir"
  git -C "$checkout_dir" remote add origin "$repository"
  git -C "$checkout_dir" sparse-checkout init --cone
  git -C "$checkout_dir" sparse-checkout set rule/Shadowrocket
  git -C "$checkout_dir" \
    -c protocol.version=2 \
    fetch --quiet --filter=blob:none --no-tags --depth=1 origin "$commit"
  git -C "$checkout_dir" checkout --quiet --detach FETCH_HEAD

  local checked_out_commit
  checked_out_commit="$(git -C "$checkout_dir" rev-parse HEAD)"
  if [[ "$checked_out_commit" != "$commit" ]]; then
    echo "expected blackmatrix7/ios_rule_script $commit, got $checked_out_commit" >&2
    return 1
  fi
}

clone_tree_paths \
  https://github.com/MetaCubeX/meta-rules-dat.git \
  meta \
  meta-rules-dat-meta
clone_tree_paths \
  https://github.com/MetaCubeX/meta-rules-dat.git \
  sing \
  meta-rules-dat-sing
checkout_repository_at_commit \
  https://github.com/blackmatrix7/ios_rule_script.git \
  "$blackmatrix_commit" \
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
