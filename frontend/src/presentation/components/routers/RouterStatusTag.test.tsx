import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { mikrotikRouter } from "@/test/mikrotikRouterFixture";
import { RouterStatusTag } from "./RouterStatusTag";

describe("RouterStatusTag", () => {
  it("flags VLANs that receive nothing next to the router's own status", () => {
    render(
      <RouterStatusTag router={mikrotikRouter({ silentInterfaces: 2 })} />,
    );

    expect(screen.getByText("Up")).toBeTruthy();
    expect(screen.getByText("2 VLAN tanpa trafik")).toBeTruthy();
  });

  it("stays quiet when every interface receives", () => {
    render(<RouterStatusTag router={mikrotikRouter()} />);

    expect(screen.queryByText(/tanpa trafik/)).toBeNull();
  });
});
