import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/infrastructure/http";
import { OltProtocol, OltModel, type Olt } from "@/domain/entities";
import { OltTable } from "./OltTable";

const discoverMutate = vi.hoisted(() => vi.fn());

vi.mock("@/application/hooks", () => ({
  useDiscoverOltNow: () => ({ mutate: discoverMutate, isPending: false }),
  useOltStats: () => ({ data: undefined, isLoading: false }),
}));

const messageSuccess = vi.hoisted(() => vi.fn());
const messageInfo = vi.hoisted(() => vi.fn());

vi.mock("antd", async () => {
  const antd = await vi.importActual<typeof import("antd")>("antd");
  return {
    ...antd,
    message: { success: messageSuccess, info: messageInfo, error: vi.fn() },
  };
});

const olt = {
  id: "olt-1",
  siteId: "site-1",
  name: "Cariu",
  ipAddress: "172.30.30.3",
  model: OltModel.ZTE_C300,
  preferredProtocol: OltProtocol.TELNET,
  status: "online",
} as Olt;

function renderTable() {
  return render(
    <MemoryRouter>
      <OltTable
        olts={[olt]}
        loading={false}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />
    </MemoryRouter>,
  );
}

describe("OltTable discover action", () => {
  beforeEach(() => {
    discoverMutate.mockReset();
    messageSuccess.mockReset();
    messageInfo.mockReset();
  });

  it("schedules an inventory pass for the row's OLT", async () => {
    renderTable();

    await userEvent.click(screen.getByRole("button", { name: /discover/i }));

    expect(discoverMutate).toHaveBeenCalledWith("olt-1", expect.anything());
  });

  it("says the pass is scheduled rather than done", async () => {
    discoverMutate.mockImplementation((_id, handlers) => handlers.onSuccess());
    renderTable();

    await userEvent.click(screen.getByRole("button", { name: /discover/i }));

    // Discovery takes minutes and the worker runs it, so the operator is told
    // it was queued, not that it finished.
    await waitFor(() =>
      expect(messageSuccess).toHaveBeenCalledWith(
        expect.stringContaining("dijadwalkan"),
      ),
    );
  });

  it("treats an already-running pass as information, not failure", async () => {
    discoverMutate.mockImplementation((_id, handlers) =>
      handlers.onError(new ApiError(409, "DISCOVERY_RUNNING")),
    );
    renderTable();

    await userEvent.click(screen.getByRole("button", { name: /discover/i }));

    // The caller asked for the pass that is already under way; refusing to
    // queue a second one is the right outcome, not an error to alarm anyone.
    await waitFor(() =>
      expect(messageInfo).toHaveBeenCalledWith(
        expect.stringContaining("sedang berjalan"),
      ),
    );
  });
});
