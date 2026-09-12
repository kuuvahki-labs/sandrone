# Generated rule-set catalog

Run `make ruleset-catalog` to generate the ignored `catalog.json.gz` artifact.
Official builds embed it; a plain `go build` without it succeeds, but the catalog
endpoint returns unavailable.

The generator derives URL metadata from the active `meta` and `sing` branches of
`MetaCubeX/meta-rules-dat` and the `master` branch's `rule/Shadowrocket` subtree of
`blackmatrix7/ios_rule_script`. For Blackmatrix it selects `.list` files named in
category README `使用说明` sections and checks their declared `RULE-SET` or
`DOMAIN-SET` shape and whether they contain only IP rules. It stores sorted,
deduplicated metadata and live URLs, not rule content. Upstream branch changes
can therefore change the catalog between builds and the content fetched later.

Set `RULESET_CATALOG_GITHUB_MIRROR` to a mirror's GitHub base URL to route build-time
clones through it. When unset, clones use `https://github.com`; generated
raw-content URLs remain canonical in either case.

Upstream projects and contributors retain attribution. Blackmatrix declares
GPL-2.0 and aggregates separately attributed rule sources; consult their licenses
and attribution when using or redistributing the referenced rule files.
