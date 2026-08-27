import { render, screen } from "@testing-library/react";
import { Form } from "antd";
import { describe, expect, it, vi } from "vitest";
import { InternetServiceForm } from "./InternetServiceForm";

const useOltVlans = vi.hoisted(() => vi.fn());

vi.mock("@/application/hooks/useOlts", () => ({ useOltVlans }));

function renderForm() {
  render(
    <Form>
      <InternetServiceForm oltId="olt-1" />
    </Form>,
  );
}

describe("InternetServiceForm", () => {
  it("offers the VLANs the OLT poll cached", () => {
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
    useOltVlans.mockReturnValue({ data: [] });

    renderForm();

    expect(
      screen.getByText("VLANs appear here once the OLT has been polled."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Select a VLAN")).not.toBeInTheDocument();
  });
});
