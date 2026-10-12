import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { mikrotikRouter } from "@/test/mikrotikRouterFixture";
import { RouterFlowStatus } from "./RouterFlowStatus";

vi.mock("@/application/hooks/useRouterFlows", () => ({
  useFlowStatus: () => ({
    data: { asnDatasetUpdatedAt: "2026-10-12T00:00:00Z", asnCount: 3 },
  }),
}));

const STALE = /tidak ada flow masuk lebih dari 5 menit/;
const minutesAgo = (m: number) =>
  new Date(Date.now() - m * 60_000).toISOString();

describe("RouterFlowStatus", () => {
  it("warns when collection is on but nothing arrived for five minutes", () => {
    render(
      <RouterFlowStatus
        router={mikrotikRouter({
          flowEnabled: true,
          flowLastSeenAt: minutesAgo(10),
        })}
      />,
    );

    expect(screen.getByText(STALE)).toBeTruthy();
  });

  it("stays quiet while flows arrive, and while collection is off", () => {
    const { unmount } = render(
      <RouterFlowStatus
        router={mikrotikRouter({
          flowEnabled: true,
          flowLastSeenAt: minutesAgo(1),
        })}
      />,
    );
    expect(screen.queryByText(STALE)).toBeNull();
    unmount();

    render(
      <RouterFlowStatus router={mikrotikRouter({ flowEnabled: false })} />,
    );

    expect(screen.queryByText(STALE)).toBeNull();
    expect(screen.getByText("Nonaktif")).toBeTruthy();
  });

  it("shows the sampling ratio and the ASN dataset in use", () => {
    render(
      <RouterFlowStatus
        router={mikrotikRouter({ flowEnabled: true, flowSampling: 512 })}
      />,
    );

    expect(screen.getByText("1:512")).toBeTruthy();
    expect(screen.getByText(/3 ASN/)).toBeTruthy();
  });
});
