import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "antd";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import OltConfigurationPage from "../OltConfigurationPage";

vi.mock("@/infrastructure/http/apiClient", () => ({
  apiClient: { get: vi.fn(), put: vi.fn(), delete: vi.fn(), post: vi.fn() },
}));

const { apiClient } = await import("@/infrastructure/http/apiClient");

const OLT_ID = "3f2a1c4e-8d6b-4f0a-9c1e-2b7d5a6e8f90";
const WA_ACCOUNT_ID = "9b1d7e2a-4c3f-4e8a-a5b6-0d2c8f1e7a43";

interface LookupRecord {
  id: string;
  name: string;
}

function renderAt(path: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <App>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route
              path="/olts/:id/configuration"
              element={<OltConfigurationPage />}
            />
          </Routes>
        </MemoryRouter>
      </App>
    </QueryClientProvider>,
  );
}

// Answers the page's OLT lookup for routeId with record, and fails every other
// read the way a missing or foreign resource would.
function answerOltLookup(routeId: string, record?: LookupRecord) {
  vi.mocked(apiClient.get).mockImplementation((async (url: string) => {
    if (record && url === API_ENDPOINTS.OLT_BY_ID(routeId)) {
      return { data: record };
    }
    throw new Error("not found");
  }) as never);
}

const refreshButton = () =>
  screen.getByRole("button", { name: /refresh now/i });

describe("OltConfigurationPage refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(apiClient.post).mockResolvedValue({
      data: { data: {} },
    } as never);
  });

  it("re-reads the OLT the server returned for this page", async () => {
    answerOltLookup(OLT_ID, { id: OLT_ID, name: "Cariu" });
    renderAt(`/olts/${OLT_ID}/configuration`);

    await screen.findByRole("heading", { name: "Cariu configuration" });
    await waitFor(() => expect(refreshButton()).toBeEnabled());
    await userEvent.click(refreshButton());

    await waitFor(() =>
      expect(apiClient.post).toHaveBeenCalledWith(
        `/api/v1/olts/${OLT_ID}/system/refresh`,
      ),
    );
  });

  // React Router hands the param back decoded, so %2F and %3F in a crafted
  // link become "../" and "?" and would carry the POST to another endpoint.
  it.each([
    {
      name: "the OLT lookup fails",
      link: "/olts/..%2Fadmin%2Fseed-events%3F/configuration",
      routeId: "../admin/seed-events?",
      record: undefined,
      heading: "OLT configuration",
    },
    {
      name: "the lookup lands on a different resource",
      link: `/olts/..%2Fcs%2Fwa-accounts%2F${WA_ACCOUNT_ID}%2Fdisconnect%3F/configuration`,
      routeId: `../cs/wa-accounts/${WA_ACCOUNT_ID}/disconnect?`,
      record: { id: WA_ACCOUNT_ID, name: "Shared WhatsApp" },
      heading: "Shared WhatsApp configuration",
    },
  ])(
    "sends no refresh when $name for a crafted id",
    async ({ link, routeId, record, heading }) => {
      answerOltLookup(routeId, record);
      renderAt(link);

      await screen.findByRole("heading", { name: heading });
      expect(refreshButton()).toBeDisabled();
      fireEvent.click(refreshButton());

      expect(apiClient.post).not.toHaveBeenCalled();
    },
  );
});
