# Generated rule-set catalog

Run `make ruleset-catalog` to generate the ignored `catalog.json.gz` artifact from
the commits recorded in `internal/tools/ruleset-catalog-gen/sources.lock`. Official
builds embed it; a plain `go build` without it succeeds, but the catalog endpoint
returns unavailable.

The manual release workflow refreshes and commits the lock before it creates a
tag. Tag CI generates the catalog while building release artifacts and container
images. The generator derives URL metadata from the locked `meta` and `sing`
commits of `MetaCubeX/meta-rules-dat` and the locked `master` commit's
`rule/Shadowrocket` subtree of `blackmatrix7/ios_rule_script`. For Blackmatrix it
selects `.list` files named in category README `使用说明` sections and checks their
declared `RULE-SET` or `DOMAIN-SET` shape and whether they contain only IP rules.

The snapshot stores sorted, deduplicated metadata and live branch URLs, not rule
content. A source-lock change therefore controls when catalog membership changes,
while content fetched from those URLs can still follow the upstream branches.

Set `RULESET_CATALOG_GITHUB_MIRROR` to a mirror's GitHub base URL to route build-time
clones through it. When unset, clones use `https://github.com`; generated
raw-content URLs remain canonical in either case.

Local CLI builds keep the two bare upstream repositories under
`${XDG_CACHE_HOME:-$HOME/.cache}/sandrone/ruleset-catalog`, so another build of the
same locked commits does not fetch them again. Set `RULESET_CATALOG_CACHE_DIR` to
change that location. Docker disables this CLI cache and uses its catalog stage's
BuildKit layer cache independently.

Upstream projects and contributors retain attribution. Blackmatrix declares
GPL-2.0 and aggregates separately attributed rule sources; consult their licenses
and attribution when using or redistributing the referenced rule files.
