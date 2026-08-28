import { describe, expect, it } from "vitest";

import {
  millisecondsToSecondsInput,
  secondsInputToMilliseconds,
  secondsInputToMillisecondsOrZero,
} from "./duration";

describe("duration input conversion", () => {
  it("shows API millisecond values as seconds", () => {
    expect(millisecondsToSecondsInput(2500)).toBe("2.5");
    expect(millisecondsToSecondsInput(0)).toBe("0");
    expect(millisecondsToSecondsInput(undefined)).toBe("");
  });

  it("converts second inputs back to API milliseconds", () => {
    expect(secondsInputToMilliseconds("2.5")).toBe(2500);
    expect(secondsInputToMilliseconds(" 3 ")).toBe(3000);
    expect(secondsInputToMilliseconds("")).toBeUndefined();
    expect(secondsInputToMillisecondsOrZero("")).toBe(0);
  });
});
