import { useCallback, useState } from "react";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PeerConfigFormat, WireguardPeerConfig } from "@/domain/entities";
import { VpnConfigModal } from "./VpnConfigModal";

// Peers whose config request fails, so a test can hold one peer's answer back
// and see what is on screen while another peer's request has not answered.
const failing = new Set<string>();

// A stand-in for usePeerConfig that keeps React Query's shape: data survives
// until the next request answers, or until reset clears it.
function useFakePeerConfig() {
  const [data, setData] = useState<WireguardPeerConfig | undefined>();
  const [isError, setIsError] = useState(false);

  const mutate = useCallback(
    ({ id, format }: { id: string; format: PeerConfigFormat }) => {
      if (failing.has(id)) {
        setIsError(true);
        return;
      }
      setIsError(false);
      setData({ format, config: `KONFIG ${id}` });
    },
    [],
  );

  const reset = useCallback(() => {
    setData(undefined);
    setIsError(false);
  }, []);

  return {
    mutate,
    reset,
    data,
    isPending: false,
    isError,
    error: new Error("gagal mengambil konfigurasi"),
  };
}

vi.mock("@/application/hooks", () => ({
  usePeerConfig: () => useFakePeerConfig(),
}));

describe("VpnConfigModal", () => {
  beforeEach(() => {
    failing.clear();
  });

  it("shows the configuration of the peer it was opened for", () => {
    render(<VpnConfigModal peerId="peer-1" onClose={vi.fn()} />);

    expect(screen.getByText("KONFIG peer-1")).toBeInTheDocument();
  });

  // The config carries the peer's private key, so one peer's must never be
  // readable under another peer's name.
  it("drops the previous peer's configuration when opened for another", () => {
    const { rerender } = render(
      <VpnConfigModal peerId="peer-1" onClose={vi.fn()} />,
    );
    expect(screen.getByText("KONFIG peer-1")).toBeInTheDocument();

    failing.add("peer-2");
    rerender(<VpnConfigModal peerId="peer-2" onClose={vi.fn()} />);

    expect(screen.queryByText("KONFIG peer-1")).not.toBeInTheDocument();
    expect(
      screen.getByText("Gagal menyiapkan konfigurasi"),
    ).toBeInTheDocument();
  });
});
