import { render, screen } from "@testing-library/react";
import { Form } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { InternetServiceForm } from "./InternetServiceForm";

const useOltVlans = vi.hoisted(() => vi.fn());
const useOltTcontProfiles = vi.hoisted(() => vi.fn());
const useOltVlanProfiles = vi.hoisted(() => vi.fn());

vi.mock("@/application/hooks/useOlts", () => ({
  useOltVlans,
  useOltTcontProfiles,
  useOltVlanProfiles,
}));

beforeEach(() => {
  useOltVlans.mockReturnValue({ data: [] });
  useOltTcontProfiles.mockReturnValue({ data: [] });
  useOltVlanProfiles.mockReturnValue({ data: [] });
});

// The form reads its own fields to decide what applies, so the wrapper has to
// supply the same starting values the modal does.
function renderForm(overrides: Record<string, unknown> = {}) {
  render(
    <Form
      initialValues={{
        vlanMode: "tag",
        serviceType: "internet",
        wanMode: "wan_ip",
        wanIpMode: "pppoe",
        ...overrides,
      }}
    >
      <InternetServiceForm oltId="olt-1" />
    </Form>,
  );
}

describe("InternetServiceForm", () => {
  it("offers the VLANs the OLT poll cached", () => {
    useOltTcontProfiles.mockReturnValue({ data: [] });
    useOltVlans.mockReturnValue({
      data: [
        { vlanId: 99, name: "VLAN0099-PPP" },
        { vlanId: 100, name: "VLAN0100" },
      ],
    });

    renderForm();

    expect(screen.getByText("Select a VLAN")).toBeInTheDocument();
    expect(
      screen.queryByText("VLANs appear here once the OLT has been polled."),
    ).not.toBeInTheDocument();
  });

  // An OLT that has never been polled, or was unreachable on its last poll, has
  // no cached list; the operator must still be able to type a VLAN ID.
  it("falls back to a typed VLAN ID when nothing is cached", () => {
    useOltTcontProfiles.mockReturnValue({ data: [] });
    useOltVlans.mockReturnValue({ data: [] });

    renderForm();

    expect(
      screen.getByText("VLANs appear here once the OLT has been polled."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Select a VLAN")).not.toBeInTheDocument();
  });

  it("offers the T-CONT profiles for both bandwidth fields", () => {
    useOltVlans.mockReturnValue({ data: [] });
    useOltTcontProfiles.mockReturnValue({ data: ["default", "1G"] });

    renderForm();

    // One for the download profile, one for the upload profile.
    expect(screen.getAllByText("Select a T-CONT profile")).toHaveLength(2);
  });

  it("falls back to typed profile names when nothing is cached", () => {
    useOltVlans.mockReturnValue({ data: [] });
    useOltTcontProfiles.mockReturnValue({ data: [] });

    renderForm();

    expect(
      screen.getByText("Profiles appear here once the OLT has been polled."),
    ).toBeInTheDocument();
  });

  // The CLI cannot list these, so the options are the names already in use on
  // the OLT's own ONUs.
  it("offers the VLAN profiles recovered from the OLT config", () => {
    useOltVlanProfiles.mockReturnValue({ data: ["PPPOE-21", "PPPOE-214"] });

    renderForm();

    expect(screen.getByText("Select a VLAN profile")).toBeInTheDocument();
  });

  // A bridged ONU carries no OMCI WAN, so the VLAN profile and credentials that
  // only describe one must not be asked for.
  it("hides the WAN fields for a bridge service", () => {
    useOltVlanProfiles.mockReturnValue({ data: ["PPPOE-214"] });

    renderForm({
      serviceType: "bridge",
      wanMode: "setup_via_ont",
      wanIpMode: "",
    });

    expect(screen.queryByText("Select a VLAN profile")).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/PPPoE password/i)).not.toBeInTheDocument();
  });

  // Untagged traffic is transport only, so it does not even offer a service type.
  it("hides the service type for an untagged service", () => {
    renderForm({ vlanMode: "untag", wanMode: "setup_via_ont", wanIpMode: "" });

    expect(screen.queryByText("Service type")).not.toBeInTheDocument();
  });

  it("drops the PPPoE credentials for a DHCP WAN", () => {
    renderForm({ wanIpMode: "dhcp" });

    expect(screen.queryByLabelText(/PPPoE username/i)).not.toBeInTheDocument();
  });

  it("falls back to a typed VLAN profile when nothing is cached", () => {
    renderForm();

    expect(
      screen.getByText(
        "VLAN profiles appear here once the OLT has been polled.",
      ),
    ).toBeInTheDocument();
  });
});
