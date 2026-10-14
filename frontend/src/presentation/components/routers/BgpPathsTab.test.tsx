import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import type { RouteAdvice, RouteLookup } from "@/domain/entities";
import { UserRole } from "@/domain/entities";
import { ApiError } from "@/infrastructure/http";
import { BgpPathsTab } from "./BgpPathsTab";

const path = {
  peer: "peer-ix",
  remoteAs: 64500,
  transit: false,
  asPath: "64500,64496",
  pathLength: 2,
  localPref: 700,
  inputFilter: "IX_IN",
};

const advice: RouteAdvice[] = [
  {
    prefix: "198.51.100.0/24",
    asn: 64496,
    name: "EXAMPLE-NET",
    downloadBytes: 5000,
    uploadBytes: 50,
    status: "optimal",
    active: path,
    alternatives: [
      {
        ...path,
        peer: "peer-transit",
        remoteAs: 64501,
        transit: true,
        asPath: "64501,64502,64496",
        pathLength: 3,
        localPref: 100,
      },
    ],
    inbound: null,
    suggested: "",
    checkedAt: "2026-10-14T03:00:00Z",
  },
  {
    prefix: "203.0.113.0/24",
    asn: 64497,
    name: "OTHER-NET",
    downloadBytes: 1000,
    uploadBytes: 10,
    status: "optimal",
    active: { ...path, asPath: "64500,64497" },
    alternatives: null,
    inbound: null,
    suggested: "",
    checkedAt: "2026-10-14T03:00:00Z",
  },
];

let role: UserRole = UserRole.ADMIN;
const lookup = vi.fn();
const asNames = vi.fn();
const state: {
  advice: { data?: RouteAdvice[]; isLoading: boolean; isError: boolean };
  lookup: { data?: RouteLookup; error: unknown; isPending: boolean };
} = {
  advice: { data: advice, isLoading: false, isError: false },
  lookup: { data: undefined, error: null, isPending: false },
};

vi.mock("@/application/stores", () => ({
  useAuthStore: (selector: (s: unknown) => unknown) =>
    selector({ user: { role } }),
}));

vi.mock("@/application/hooks/useRouterFlows", () => ({
  useFlowRouteAdvice: () => state.advice,
}));

vi.mock("@/application/hooks/useRouterPaths", () => ({
  useAsNames: (...args: unknown[]) => {
    asNames(...args);
    return { data: { "64500": "IX-RS", "64502": "TRANSIT-CORE" } };
  },
  useRouteLookup: () => ({
    mutate: lookup,
    ...state.lookup,
    isError: state.lookup.error != null,
  }),
}));

function edges(container: HTMLElement, active?: boolean) {
  const selector =
    active === undefined
      ? "path[data-edge]"
      : `path[data-edge][data-active="${active}"]`;
  return container.querySelectorAll(selector);
}

describe("BgpPathsTab", () => {
  beforeEach(() => {
    role = UserRole.ADMIN;
    lookup.mockReset();
    asNames.mockReset();
    state.advice = { data: advice, isLoading: false, isError: false };
    state.lookup = { data: undefined, error: null, isPending: false };
  });

  it("draws the active path of each busy destination, named from both sources", () => {
    const { container } = render(
      <BgpPathsTab routerId="r1" routerName="Border" />,
    );

    const overview = screen.getByTestId("path-overview");
    expect(within(overview).getByText("IX-RS")).toBeInTheDocument();
    expect(within(overview).getByText("EXAMPLE-NET")).toBeInTheDocument();
    expect(within(overview).getByText("OTHER-NET")).toBeInTheDocument();
    expect(edges(container, false)).toHaveLength(0);
    expect(edges(container, true)).toHaveLength(3);
    expect(asNames).toHaveBeenCalledWith("r1", [64496, 64497, 64500]);
  });

  it("adds the alternatives, dashed, on request", () => {
    const { container } = render(
      <BgpPathsTab routerId="r1" routerName="Border" />,
    );

    fireEvent.click(screen.getByRole("switch", { name: /alternatif/i }));

    expect(edges(container, false)).toHaveLength(3);
    expect(screen.getByText("TRANSIT-CORE")).toBeInTheDocument();
  });

  it("lists the prefixes of a destination AS when its node is clicked", () => {
    render(<BgpPathsTab routerId="r1" routerName="Border" />);

    fireEvent.click(screen.getByText("EXAMPLE-NET"));

    const detail = screen.getByTestId("path-detail");
    expect(within(detail).getByText("198.51.100.0/24")).toBeInTheDocument();
    expect(within(detail).queryByText("203.0.113.0/24")).toBeNull();
    expect(within(detail).getByText("64501,64502,64496")).toBeInTheDocument();
  });

  it("asks the router for the target typed and draws what it answers", () => {
    render(<BgpPathsTab routerId="r1" routerName="Border" />);
    fireEvent.change(screen.getByPlaceholderText(/IP atau prefix/i), {
      target: { value: " 198.51.100.7 " },
    });
    fireEvent.click(screen.getByRole("button", { name: /cek di router/i }));
    expect(lookup).toHaveBeenCalledWith("198.51.100.7");

    state.lookup = {
      data: {
        target: "198.51.100.7",
        prefix: "198.51.100.0/23",
        queries: 2,
        checkedAt: "2026-10-14T04:00:00Z",
        routes: [
          {
            session: "peer-ix-1",
            asPath: "64500,64496",
            localPref: 700,
            active: true,
            gateway: "192.0.2.1",
          },
          {
            session: "peer-transit-1",
            asPath: "64501,64496",
            localPref: 100,
            active: false,
            gateway: "192.0.2.9",
          },
        ],
      },
      error: null,
      isPending: false,
    };
    render(<BgpPathsTab routerId="r1" routerName="Border" />);

    const result = screen.getByTestId("lookup-result");
    expect(within(result).getByText(/198\.51\.100\.0\/23/)).toBeInTheDocument();
    expect(within(result).getAllByText("peer-transit-1")).toHaveLength(2);
    expect(within(result).getByText("AS64496")).toBeInTheDocument();
  });

  it("explains a router with no route for the target", () => {
    state.lookup = {
      data: {
        target: "203.0.113.1",
        prefix: "",
        queries: 17,
        checkedAt: "2026-10-14T04:00:00Z",
        routes: [],
      },
      error: null,
      isPending: false,
    };
    render(<BgpPathsTab routerId="r1" routerName="Border" />);

    expect(screen.getByText(/tidak punya rute BGP/i)).toBeInTheDocument();
  });

  it("tells the operator when another lookup holds the router", () => {
    state.lookup = {
      data: undefined,
      error: new ApiError(409, "LOOKUP_BUSY"),
      isPending: false,
    };
    render(<BgpPathsTab routerId="r1" routerName="Border" />);

    expect(screen.getByText(/sedang berjalan/i)).toBeInTheDocument();
  });

  it("keeps the live lookup from a viewer", () => {
    role = UserRole.VIEWER;
    render(<BgpPathsTab routerId="r1" routerName="Border" />);

    expect(screen.queryByRole("button", { name: /cek di router/i })).toBeNull();
    expect(screen.getByText(/admin dan teknisi/i)).toBeInTheDocument();
  });
});
