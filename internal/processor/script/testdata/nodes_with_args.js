function main(input, api) {
  var prefix = (input.args && input.args.prefix) || "";
  input.nodes.forEach(function(n) {
    n.name = prefix + n.name;
  });
  return input;
}
