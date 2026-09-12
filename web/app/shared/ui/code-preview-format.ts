// Format presentation text only. Never serialize parsed values: doing so can
// round large numbers, remove duplicate keys, or rewrite string escapes.
export function formatCodePreview(value: string, language: string): string {
  if (language.toLowerCase() !== "json") return value;

  try {
    // Validation only; the parsed value is deliberately discarded.
    JSON.parse(value);
  } catch {
    return value;
  }

  const tokens = value.match(/"(?:\\.|[^"\\])*"|[{}[\],:]|[^\s{}[\],:]+/g) ?? [];
  const output: string[] = [];
  let depth = 0;
  const newline = () => output.push(`\n${"  ".repeat(depth)}`);

  for (let index = 0; index < tokens.length; index++) {
    const token = tokens[index];
    switch (token) {
      case "{":
      case "[":
        output.push(token);
        if (tokens[index + 1] === (token === "{" ? "}" : "]")) {
          output.push(tokens[++index]);
        } else {
          depth++;
          // Deeply nested input can otherwise expand quadratically in size.
          if (depth > 128) return value;
          newline();
        }
        break;
      case "}":
      case "]":
        depth--;
        newline();
        output.push(token);
        break;
      case ",":
        output.push(token);
        newline();
        break;
      case ":":
        output.push(": ");
        break;
      default:
        output.push(token);
    }
  }

  return output.join("");
}
