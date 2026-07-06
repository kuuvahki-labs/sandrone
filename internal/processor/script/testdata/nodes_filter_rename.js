function main(input, api) {
  api.log("processing", input.nodes.length, "nodes");
  input.nodes = input.nodes.filter(function (node) {
    return node.type !== "http";
  });
  input.nodes.forEach(function (node) {
    node.name = "[" + (input.target || "any") + "]" + node.name;
    node.ext = node.ext || {};
    node.ext.touched_by = "rename-script";
  });
  return input;
}
