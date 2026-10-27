// Cloudflare trace example: request first, then mutate/filter the input nodes.
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
  var candidates = [];
  results.forEach(function (result) {
    var fields = {};
    var duplicate = false;
    result.body.split(/\r?\n/).forEach(function (line) {
      var separator = line.indexOf("=");
      if (separator > 0) {
        var key = line.slice(0, separator);
        if (Object.prototype.hasOwnProperty.call(fields, key)) duplicate = true;
        fields[key] = line.slice(separator + 1).trim();
      }
    });
    if (result.error || duplicate || result.status_code !== 200 || !publicIP(fields.ip) || !validCountry(fields.loc)) {
      api.warn({code: "exit_info_invalid", message: "No valid exit IP/country; node removed", node_index: result.index});
      return;
    }
    var node = input.nodes[result.index];
    var identity = {};
    Object.keys(node).forEach(function (key) {
      if (["name", "meta", "tags", "raw", "ext", "warnings", "lossy", "source_format"].indexOf(key) < 0) identity[key] = node[key];
    });
    var key = canonical(identity);
    var hash = api.hash.sha256(key);
    candidates.push({node: node, ip: fields.ip, country: fields.loc, key: key, hash: hash, index: result.index});
  });
  // Resolve rare six-character hash collisions in a stable order, independently
  // of response completion order. Preserve original order in the final output.
  candidates.sort(function (a, b) { return a.key < b.key ? -1 : a.key > b.key ? 1 : a.index - b.index; });
  var counts = {};
  candidates.forEach(function (item) {
    var base = item.country + "-" + item.hash.slice(0, 6);
    counts[base] = (counts[base] || 0) + 1;
    item.node.name = base + (counts[base] === 1 ? "" : "-" + counts[base]);
    item.node.meta = item.node.meta || {};
    item.node.meta["exit.ip"] = item.ip;
    item.node.meta["exit.country"] = item.country;
  });
  candidates.sort(function (a, b) { return a.index - b.index; });
  input.nodes = candidates.map(function (item) { return item.node; });
  return input;
}

function canonical(value) {
  if (Array.isArray(value)) return "[" + value.map(canonical).join(",") + "]";
  if (value && typeof value === "object") return "{" + Object.keys(value).sort().map(function (key) { return JSON.stringify(key) + ":" + canonical(value[key]); }).join(",") + "}";
  return JSON.stringify(value);
}

function validIP(ip) {
  if (typeof ip !== "string") return false;
  function ipv4(value) {
    var parts = value.split(".");
    return parts.length === 4 && parts.every(function (part) { return /^(0|[1-9][0-9]{0,2})$/.test(part) && +part <= 255; });
  }
  if (ip.indexOf(":") < 0) return ipv4(ip);
  if (ip.indexOf(".") >= 0) {
    var last = ip.lastIndexOf(":");
    if (!ipv4(ip.slice(last + 1))) return false;
    ip = ip.slice(0, last + 1) + "0:0";
  }
  var halves = ip.split("::");
  if (halves.length > 2) return false;
  var groups = [];
  halves.forEach(function (half) { if (half) groups = groups.concat(half.split(":")); });
  return groups.every(function (group) { return /^[0-9a-fA-F]{1,4}$/.test(group); }) &&
    (halves.length === 2 ? groups.length < 8 : groups.length === 8);
}

function validCountry(country) {
  // ISO 3166-1 alpha-2; unknown markers such as XX and T1 are deliberately removed.
  return typeof country === "string" && country.length === 2 && (
    " AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS YE YT ZA ZM ZW "
  ).indexOf(" " + country + " ") >= 0;
}

// Reject common non-public ranges; successful observation still does not prove
// allocation, geolocation accuracy, or that an address will remain reachable.
function publicIP(ip) {
  if (!validIP(ip)) return false;
  function publicV4(parts) {
    var a = +parts[0], b = +parts[1], c = +parts[2];
    return !(a === 0 || a === 10 || a === 127 || a >= 224 ||
      (a === 100 && b >= 64 && b <= 127) || (a === 169 && b === 254) ||
      (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168) ||
      (a === 192 && b === 0 && (c === 0 || c === 2)) ||
      (a === 198 && (b === 18 || b === 19 || (b === 51 && c === 100))) ||
      (a === 203 && b === 0 && c === 113));
  }
  if (ip.indexOf(":") < 0) return publicV4(ip.split("."));
  if (ip.indexOf(".") >= 0) {
    var last = ip.lastIndexOf(":");
    var v4 = ip.slice(last + 1).split(".");
    ip = ip.slice(0, last + 1) + ((+v4[0] << 8) + +v4[1]).toString(16) + ":" + ((+v4[2] << 8) + +v4[3]).toString(16);
  }
  var halves = ip.split("::");
  var left = halves[0] ? halves[0].split(":") : [];
  var right = halves.length === 2 && halves[1] ? halves[1].split(":") : [];
  if (halves.length === 2) while (left.length + right.length < 8) left.push("0");
  var groups = left.concat(right).map(function (group) { return parseInt(group, 16); });
  if (groups.slice(0, 5).every(function (group) { return group === 0; }) && groups[5] === 65535)
    return publicV4([groups[6] >> 8, groups[6] & 255, groups[7] >> 8, groups[7] & 255]);
  return (groups[0] & 0xe000) === 0x2000 && !(groups[0] === 0x2001 && groups[1] === 0xdb8) &&
    !(groups[0] === 0x3fff && groups[1] < 0x1000);
}
