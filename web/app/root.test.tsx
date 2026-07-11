import { describe, expect, it } from "vitest";

import { Layout, links } from "./root";

describe("root layout", () => {
  it("allows the MUI color scheme script to update html attributes before hydration", () => {
    const root = Layout({ children: null });

    expect(root.type).toBe("html");
    expect(root.props.suppressHydrationWarning).toBe(true);
  });

  it("exposes Sandrone favicon assets", () => {
    expect(links()).toEqual(expect.arrayContaining([
      { rel: "icon", href: "/brand/favicon.ico", sizes: "any" },
      { rel: "icon", href: "/brand/sandrone-logo-32.png", sizes: "32x32", type: "image/png" },
      { rel: "icon", href: "/brand/sandrone-logo-48.png", sizes: "48x48", type: "image/png" },
    ]));
  });
});
