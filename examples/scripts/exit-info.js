// Cloudflare trace example: annotate nodes using one timed exit-info request.
// The request is through each proxy; this does not establish future availability.
function main(input, api) {
  var args = input.args || {};
  var results = api.http.request(input.nodes, {
    url: args.url || "https://www.cloudflare.com/cdn-cgi/trace",
    core: args.core || "sing-box",
    method: "GET",
    timeout_ms: args.timeout_ms || 5000,
    concurrency: args.concurrency || 4,
    max_response_bytes: 65536
  });
  var byIndex = [];
  results.forEach(function (result) { byIndex[result.index] = result; });
  // Update observations only after the exact input nodes have been requested.
  input.nodes.forEach(function (node, index) {
    node.meta = node.meta || {};
    delete node.meta["exit.ip"];
    delete node.meta["exit.country"];
    delete node.meta["exit.duration_ms"];
    node.meta["exit.valid"] = "false";
    var result = byIndex[index];
    var fields = {};
    var duplicate = false;
    if (result && typeof result.body === "string") result.body.split(/\r?\n/).forEach(function (line) {
      var separator = line.indexOf("=");
      if (separator > 0) {
        var key = line.slice(0, separator);
        if (Object.prototype.hasOwnProperty.call(fields, key)) duplicate = true;
        fields[key] = line.slice(separator + 1).trim();
      }
    });
    if (!result || result.error || duplicate || result.status_code !== 200 ||
        typeof result.duration_ms !== "number" || !isFinite(result.duration_ms) || result.duration_ms < 0) {
      api.warn({code: "exit_info_invalid", message: "No valid exit IP/country/duration observation", node_index: index});
      return;
    }
    node.meta["exit.valid"] = "true";
    node.meta["exit.ip"] = fields.ip;
    node.meta["exit.country"] = fields.loc;
    node.meta["exit.duration_ms"] = String(result.duration_ms);
  });
  return input;
}
