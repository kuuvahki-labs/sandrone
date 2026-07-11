import "@testing-library/jest-dom/vitest";

import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

Object.defineProperty(navigator, "languages", {
  configurable: true,
  value: ["zh-CN"],
});

Object.defineProperty(navigator, "language", {
  configurable: true,
  value: "zh-CN",
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});
