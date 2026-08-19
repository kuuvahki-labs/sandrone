# Generated rule-set catalog

`make ruleset-catalog` derives URL metadata from the active `meta` and `sing`
Git branches of `MetaCubeX/meta-rules-dat`, plus the active `master` branch's
`rule/Shadowrocket` subtree of `blackmatrix7/ios_rule_script`. The Blackmatrix
generator includes only `.list` files named in each category README's
`使用说明` section. It reads the artifacts only to validate their declared
`RULE-SET` or `DOMAIN-SET` shape and to distinguish IP-only rule sets; no rule
content is stored in the catalog.

Generated Blackmatrix URLs use live `master` raw-content paths. Catalog
metadata therefore follows the active upstream branches and can change between
builds. The generator sorts and deduplicates all targets before installing
`catalog.json.gz` here.

Local builds can route the source repository clones through a GitHub mirror by
setting `RULESET_CATALOG_GITHUB_MIRROR` to the mirror's GitHub base URL before
running `make ruleset-catalog`. When the variable is unset, including in CI,
the generator clones directly from `https://github.com`. This setting affects
only the build-time clones; generated raw-content URLs remain canonical.

The source projects and their contributors retain attribution for the upstream
metadata. `blackmatrix7/ios_rule_script` declares GPL-2.0 for its repository and
also aggregates rule data from separately attributed upstream sources; those
rule sources may carry additional or different terms. Review upstream licenses
and attribution before fetching, using, or redistributing the referenced rule
files. This catalog redistributes metadata and live URLs only.

The gzip is an ignored build artifact. Official builds embed it; a plain
`go build` without it remains valid, but the catalog endpoint returns
unavailable.
