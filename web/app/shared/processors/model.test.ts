import { describe, expect, it, vi } from "vitest";

import {
  arrayValue,
  cleanParams,
  createProcessorDraftId,
  customProcessorName,
  keyValueTextToObject,
  keyValueTextToReplacementPatch,
  linesToList,
  listToLines,
  listToText,
  numberInputValue,
  numberOrEmpty,
  objectToKeyValueText,
  stringValue,
  textToList,
} from "./model";

describe("processor parameter helpers", () => {
  it("creates IDs with the requested stable prefix and index", () => {
    const random = vi.spyOn(Math, "random").mockReturnValue(0.5);

    expect(createProcessorDraftId("processor", 7)).toBe("processor-7-i");
    random.mockRestore();
  });

  it("removes empty params while preserving meaningful falsy values", () => {
    expect(cleanParams({
      emptyArray: [],
      emptyObject: {},
      emptyString: "",
      falseValue: false,
      nullValue: null,
      number: 0,
      presentArray: ["value"],
      presentObject: { key: "value" },
      undefinedValue: undefined,
    })).toEqual({
      falseValue: false,
      number: 0,
      presentArray: ["value"],
      presentObject: { key: "value" },
    });
  });

  it("uses only custom processor names that differ from the type label", () => {
    const labelForType = (type: string) => type === "script" ? "脚本" : type;

    expect(customProcessorName({ id: "1", name: " 自定义 ", type: "script", params: {} }, labelForType)).toBe("自定义");
    expect(customProcessorName({ id: "2", name: "脚本", type: "script", params: {} }, labelForType)).toBe("");
  });

  it("converts scalar and list editor values without retaining empty entries", () => {
    expect(stringValue("value")).toBe("value");
    expect(stringValue(42)).toBe("");
    expect(arrayValue(["one", 2, ""])).toEqual(["one", "2"]);
    expect(arrayValue("one")).toEqual([]);
    expect(listToText(["one", "two"])).toBe("one, two");
    expect(textToList(" one, , two ")).toEqual(["one", "two"]);
    expect(listToLines(["one", "two"])).toBe("one\ntwo");
    expect(linesToList("one\r\ntwo, three")).toEqual(["one", "two", "three"]);
  });

  it("converts finite number fields and leaves invalid or empty values blank", () => {
    expect(numberInputValue(12)).toBe(12);
    expect(numberInputValue(Number.NaN)).toBe("");
    expect(numberInputValue("12")).toBe("");
    expect(numberOrEmpty(" 12.5 ")).toBe(12.5);
    expect(numberOrEmpty(" ")).toBe("");
    expect(numberOrEmpty("invalid")).toBe("");
  });

  it("round-trips scalar and structured key-value params", () => {
    const params = keyValueTextToObject([
      "enabled=true",
      "disabled=false",
      "count=2",
      'object={"name":"node"}',
      "list=[1,2]",
      "plain=value",
      "ignored",
    ].join("\n"));

    expect(params).toEqual({
      count: 2,
      disabled: false,
      enabled: true,
      list: [1, 2],
      object: { name: "node" },
      plain: "value",
    });
    expect(objectToKeyValueText(params)).toBe([
      "enabled=true",
      "disabled=false",
      "count=2",
      'object={"name":"node"}',
      "list=[1,2]",
      "plain=value",
    ].join("\n"));
    expect(objectToKeyValueText(null)).toBe("");
  });

  it("replaces stale keys when parsing key-value text", () => {
    expect(keyValueTextToReplacementPatch({ stale: "old" }, "next=value")).toEqual({
      next: "value",
      stale: undefined,
    });
  });
});
