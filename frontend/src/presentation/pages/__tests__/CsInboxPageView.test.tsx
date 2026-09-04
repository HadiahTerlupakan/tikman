import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import { createMemoryRouter, Outlet, RouterProvider } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { UserRole } from "@/domain/entities";

// CsInboxPage pulls in the whole inbox: a dozen queries and mutations, the
// auth store, and the layout's event stream. None of that is what this file is
// about — it is about which view the segmented control shows — so the hooks are
// stubbed down to the shapes the page destructures.
vi.mock("@/application/hooks", () => {
  // CsInboxPage and the components under it pull dozens of hooks from this
  // barrel, and none of them is what this file is about — it is about which
  // view the segmented control shows. One stub carrying every field the call
  // sites destructure answers all of them. Vitest checks a mock's exports
  // against the real module, so they are listed rather than proxied.
  const stub = {
    data: [],
    isLoading: false,
    isPending: false,
    mutate: vi.fn(),
    mutateAsync: vi.fn().mockResolvedValue(undefined),
    permission: "unsupported" as const,
    requesting: false,
    enable: vi.fn(),
  };
  return {
    useAssignConversation: () => stub,
    useAssignOntToOdp: () => stub,
    useBrowserSettings: () => stub,
    useClearCsConversation: () => stub,
    useClearCsInbox: () => stub,
    useClearWaAccountMessages: () => stub,
    useConfigTemplate: () => stub,
    useConfigTemplates: () => stub,
    useConnectWaAccount: () => stub,
    useCreateConfigTemplate: () => stub,
    useCreateOdc: () => stub,
    useCreateOdp: () => stub,
    useCreateOlt: () => stub,
    useCreateOnt: () => stub,
    useCreateQuickReply: () => stub,
    useCreateSite: () => stub,
    useCreateUser: () => stub,
    useCreateWaAccount: () => stub,
    useCreateWireguardPeer: () => stub,
    useCsConversations: () => stub,
    useCsHistory: () => stub,
    useCsQuickReplies: () => stub,
    useCsStream: () => stub,
    useCurrentUser: () => stub,
    useDashboardStats: () => stub,
    useDeleteConfigTemplate: () => stub,
    useDeleteCsMessage: () => stub,
    useDeleteOlt: () => stub,
    useDeleteOnt: () => stub,
    useDeleteQuickReply: () => stub,
    useDeleteSetting: () => stub,
    useDeleteSite: () => stub,
    useDeleteUser: () => stub,
    useDeleteWaAccount: () => stub,
    useDeleteWireguardPeer: () => stub,
    useDisconnectWaAccount: () => stub,
    useDiscoverOltNow: () => stub,
    useGoogleMapsKey: () => stub,
    useHealth: () => stub,
    useLinkConversationOnt: () => stub,
    useLogin: () => stub,
    useLogout: () => stub,
    useOdcFeeds: () => stub,
    useOdcs: () => stub,
    useOdpSubscribers: () => stub,
    useOdps: () => stub,
    useOlt: () => stub,
    useOltAggregateTraffic: () => stub,
    useOltOnuTypes: () => stub,
    useOltStats: () => stub,
    useOltSystem: () => stub,
    useOltTcontProfiles: () => stub,
    useOltTopology: () => stub,
    useOltVlanProfiles: () => stub,
    useOltVlans: () => stub,
    useOlts: () => stub,
    useOnt: () => stub,
    useOntAvailability: () => stub,
    useOntEvents: () => stub,
    useOntMetrics: () => stub,
    useOntMetricsHistory: () => stub,
    useOntMetricsRealtime: () => stub,
    useOntRemovalPreview: () => stub,
    useOntServiceConfig: () => stub,
    useOntTrafficTimeSeries: () => stub,
    useOnts: () => stub,
    usePeerConfig: () => stub,
    usePonHealth: () => stub,
    useProvisionJob: () => stub,
    useProvisionJobsByONT: () => stub,
    useProvisionOnt: () => stub,
    usePushNotifications: () => stub,
    useRefreshOltSystem: () => stub,
    useSaveSetting: () => stub,
    useSaveWireguardServer: () => stub,
    useSendCsMedia: () => stub,
    useSendCsMessage: () => stub,
    useSetCableRoute: () => stub,
    useSetConversationStatus: () => stub,
    useSettings: () => stub,
    useSite: () => stub,
    useSites: () => stub,
    useSuggestedSubnets: () => stub,
    useTestReachability: () => stub,
    useTroubledOnts: () => stub,
    useUnassignOntFromOdp: () => stub,
    useUnconfiguredOnus: () => stub,
    useUpdateConfigTemplate: () => stub,
    useUpdateOlt: () => stub,
    useUpdateOnt: () => stub,
    useUpdateQuickReply: () => stub,
    useUpdateSite: () => stub,
    useUpdateUser: () => stub,
    useUpdateWireguardPeer: () => stub,
    useUser: () => stub,
    useUsers: () => stub,
    useWaAccounts: () => stub,
    useWireguardPeers: () => stub,
    useWireguardServer: () => stub,
    useZteCommandPreview: () => stub,
    useZteExistingService: () => stub,
    useZteGPONRegister: () => stub,
    useZteProvisionJob: () => stub,
  };
});

vi.mock("@/application/stores", () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({
      user: {
        id: "u1",
        username: "admin",
        email: "admin@tikman.local",
        initials: "AD",
        role: UserRole.ADMIN,
      },
    }),
}));

import { CsInboxPage } from "../CsInboxPage";

function renderAt(url: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const router = createMemoryRouter(
    [
      {
        path: "/cs",
        element: <Outlet context={{ csStream: {} }} />,
        children: [{ index: true, element: <CsInboxPage /> }],
      },
    ],
    { initialEntries: [url] },
  );
  return render(
    React.createElement(
      QueryClientProvider,
      { client },
      <RouterProvider router={router} />,
    ),
  );
}

/** The selected option in antd's Segmented is the label carrying
 * `ant-segmented-item-selected`. Reading the class rather than a role because
 * the control renders its inputs without an explicit radio role — verified
 * against the rendered DOM rather than assumed. */
function selectedView(): string | undefined {
  return (
    document
      .querySelector(".ant-segmented-item-selected")
      ?.textContent?.trim() ?? undefined
  );
}

describe("CsInboxPage view", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("opens on the view the URL names", () => {
    renderAt("/cs?view=belum-dibalas");
    expect(selectedView()).toBe("Belum dibalas");
  });

  it("falls back to Semua when the URL names no real view", () => {
    renderAt("/cs?view=tidak-ada");
    expect(selectedView()).toBe("Semua");
  });

  it("defaults to Semua with no view in the URL", () => {
    renderAt("/cs");
    expect(selectedView()).toBe("Semua");
  });

  // The regression this file exists for. The navbar bell links to
  // ?view=belum-dibalas, and a CS clicking it while already on this page
  // navigates within the same route — nothing remounts. When the view was
  // component state seeded from the URL, the initial value was read once and
  // never again, so the click changed the address bar and nothing else, which
  // read as a bell that could not be clicked at all.
  it("follows the URL when it changes without a remount", async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const router = createMemoryRouter(
      [
        {
          path: "/cs",
          element: <Outlet context={{ csStream: {} }} />,
          children: [{ index: true, element: <CsInboxPage /> }],
        },
      ],
      { initialEntries: ["/cs"] },
    );
    render(
      React.createElement(
        QueryClientProvider,
        { client },
        <RouterProvider router={router} />,
      ),
    );
    expect(selectedView()).toBe("Semua");

    await router.navigate("/cs?view=belum-dibalas");

    await waitFor(() => expect(selectedView()).toBe("Belum dibalas"));
  });
});
