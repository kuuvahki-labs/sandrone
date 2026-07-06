function main(input, api) {
  var doc = api.yaml.parse(input.file.content);
  if (!doc["proxy-groups"]) {
    doc["proxy-groups"] = [];
  }
  var nodePart = null;
  var parts = input.parts || [];
  for (var i = 0; i < parts.length; i++) {
    if (parts[i].role === "nodes") {
      nodePart = parts[i];
      break;
    }
  }
  var nodeNames = [];
  if (nodePart && nodePart.nodes) {
    for (var j = 0; j < nodePart.nodes.length; j++) {
      nodeNames.push(nodePart.nodes[j].name);
    }
  }
  doc["proxy-groups"].push({
    name: "AUTO",
    type: "url-test",
    proxies: nodeNames,
    url: "https://www.gstatic.com/generate_204",
    interval: 300
  });
  input.file.content = api.yaml.stringify(doc);
  return input;
}
