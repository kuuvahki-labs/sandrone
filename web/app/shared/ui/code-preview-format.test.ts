import { describe, expect, it } from "vitest";

import { formatCodePreview } from "./code-preview-format";

describe("formatCodePreview", () => {
  it("indents nested JSON with two spaces while keeping empty containers compact", () => {
    const source = ' \r\n { "name":"example", "items": [1, {"enabled":true,"empty": [ ]}], "options": { }, "missing": null }\t';
    const expected = [
      "{",
      '  "name": "example",',
      '  "items": [',
      "    1,",
      "    {",
      '      "enabled": true,',
      '      "empty": []',
      "    }",
      "  ],",
      '  "options": {},',
      '  "missing": null',
      "}",
    ].join("\n");

    expect(formatCodePreview(source, "json")).toBe(expected);
    expect(formatCodePreview(expected, "json")).toBe(expected);
  });

  it("preserves large integers, numeric spelling, duplicate keys, and original key order", () => {
    const source = '{"10":9007199254740993123456789,"2":-0,"n":1.2300e+04,"n":1E-007,"overflow":1e400}';
    const expected = [
      "{",
      '  "10": 9007199254740993123456789,',
      '  "2": -0,',
      '  "n": 1.2300e+04,',
      '  "n": 1E-007,',
      '  "overflow": 1e400',
      "}",
    ].join("\n");

    expect(formatCodePreview(source, "json")).toBe(expected);
    expect(formatCodePreview(expected, "json")).toBe(expected);
  });

  it("preserves string escapes and whitespace inside strings without interpreting their punctuation", () => {
    const source = String.raw`{"\u0061":"  { [ : , ] }  ","escaped":"quote: \" slash: \/ backslash: \\ newline: \n tab: \t unicode: \u00e9","tail\\":"中文"}`;
    const expected = [
      "{",
      String.raw`  "\u0061": "  { [ : , ] }  ",`,
      String.raw`  "escaped": "quote: \" slash: \/ backslash: \\ newline: \n tab: \t unicode: \u00e9",`,
      String.raw`  "tail\\": "中文"`,
      "}",
    ].join("\n");

    expect(formatCodePreview(source, "json")).toBe(expected);
    expect(formatCodePreview(expected, "json")).toBe(expected);
  });

  it.each([
    [" \n { \t } \r", "{}"],
    [" \n [ \t ] \r", "[]"],
    [" \n true \t", "true"],
    [" \n null \t", "null"],
    [" \n -0.000e+07 \t", "-0.000e+07"],
    [' \n "  value  " \t', '"  value  "'],
  ])("formats top-level values without changing their spelling: %j", (source, expected) => {
    expect(formatCodePreview(source, "json")).toBe(expected);
    expect(formatCodePreview(expected, "json")).toBe(expected);
  });

  it.each([
    "",
    " \r\n\t",
    ' {"value":1,} ',
    ' {"value":01} ',
    ' {"value":NaN} ',
    ' {"value":1} trailing ',
    ' {"value":1} {} ',
    ' {"value":[1,2} ',
    ' {"value":"unterminated} ',
    String.raw` {"value":"\q"} `,
    ' {"value":"line\nbreak"} ',
    ' /* comment */ {"value":1} ',
    '\u00a0{"value":1}',
  ])("returns invalid JSON unchanged, including its original whitespace: %j", (source) => {
    expect(formatCodePreview(source, "json")).toBe(source);
  });

  it.each(["text", "yaml", "javascript", "json-diff", "ini"])("leaves %s content unchanged even when it is valid JSON", (language) => {
    const source = ' \r\n { "value" : 1 } \t';

    expect(formatCodePreview(source, language)).toBe(source);
  });

  it("preserves extremely nested JSON instead of expanding its indentation", () => {
    const source = ` \n${"[".repeat(1000)}0${"]".repeat(1000)}\t`;

    expect(formatCodePreview(source, "json")).toBe(source);
  });
});
