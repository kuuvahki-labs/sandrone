import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { WarningList } from "~/shared/resources/warnings";

describe("warning list", () => {
  it("shows the warning message and structured context without interpreting it", () => {
    render(
      <WarningList
        warnings={[{
          code: "parse_unknown_field",
          field: "uri.query.mode",
          message: "field preserved in NodeIR Raw",
          node: "[vless]剩余流量：94.89 GB",
          source: "uri-list",
        }]}
      />,
    );

    expect(screen.getByRole("heading", { name: "parse_unknown_field · field preserved in NodeIR Raw" })).toBeInTheDocument();
    expect(screen.getByText("[vless]剩余流量：94.89 GB").parentElement?.parentElement).toHaveTextContent(
      "node: [vless]剩余流量：94.89 GB · source: uri-list · field: uri.query.mode",
    );
  });
});
