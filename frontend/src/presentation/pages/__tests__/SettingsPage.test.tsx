import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { App } from "antd";
import SettingsPage from "../SettingsPage";

// SettingsPage takes message and modal from App.useApp(). Rendering it bare
// falls back to antd's statics and warns, so the provider goes in here.
const renderPage = () =>
  render(
    <App>
      <SettingsPage />
    </App>,
  );

const state: {
  data: unknown;
  isLoading: boolean;
} = { data: [], isLoading: false };

vi.mock("@/application/hooks", () => ({
  useSettings: () => state,
  useSaveSetting: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteSetting: () => ({ mutate: vi.fn(), isPending: false }),
}));

describe("SettingsPage", () => {
  beforeEach(() => {
    state.data = [
      {
        name: "google_maps_api_key",
        label: "Google Maps API key",
        description: "Enables the site map.",
        configured: false,
        preview: "",
      },
    ];
    state.isLoading = false;
  });

  it("lists every known setting, including ones never configured", () => {
    renderPage();

    expect(screen.getByText("Google Maps API key")).toBeInTheDocument();
    expect(screen.getByText("Not configured")).toBeInTheDocument();
  });

  it("keeps the key restriction steps on screen beside the Maps key", () => {
    // An operator who believes the key is secret will not go and restrict it,
    // so this guidance is permanent rather than a dismissible hint.
    renderPage();

    expect(screen.getByText(/Application restrictions/i)).toBeInTheDocument();
    expect(screen.getByText(/your-noc-domain/)).toBeInTheDocument();
  });
});
