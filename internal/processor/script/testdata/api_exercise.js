function main(input, api) {
  api.log("hello");
  api.warn({ code: "custom_warn", message: "from script" });
  var parsed = api.yaml.parse("key: value\n");
  var yamlOut = api.yaml.stringify(parsed);
  var jsonDoc = api.json.parse('{"a":1}');
  var jsonOut = api.json.stringify(jsonDoc);
  var enc = api.base64.encode("foo");
  var dec = api.base64.decode(enc);
  var digest = api.hash.sha256("foo");
  input.nodes.forEach(function(n) {
    n.name = dec + ":" + digest.slice(0, 8);
    n.ext = { yaml_len: yamlOut.length, json_len: jsonOut.length };
  });
  return input;
}
