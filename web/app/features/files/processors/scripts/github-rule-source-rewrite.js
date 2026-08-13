/* eslint-disable unused-imports/no-unused-vars */

// sandrone:file-preset=github-rule-source-rewrite
// Edit the destination values below to use another mirror.
const replacements = [
  [
    "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/",
    "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@meta/",
  ],
  [
    "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/",
    "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/",
  ],
  [
    "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/",
  ],
];

function main(input) {
  if (!input.file || typeof input.file.content !== "string") {
    return input;
  }
  let content = input.file.content;
  for (const [source, destination] of replacements) {
    content = content.split(source).join(destination);
  }
  input.file.content = content;
  return input;
}
