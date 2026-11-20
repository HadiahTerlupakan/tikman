import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { SwitchFormModal } from "./SwitchFormModal";
import { TELNET_WARNING, protocolsFor } from "./switchProtocol";

const testMutate = vi.fn();
const createMutate = vi.fn();
const updateMutate = vi.fn();

vi.mock("@/application/hooks/useSwitches", () => ({
  useTestSwitchConnection: () => ({
    mutateAsync: testMutate,
    isPending: false,
  }),
  useCreateSwitch: () => ({ mutateAsync: createMutate, isPending: false }),
  useUpdateSwitch: () => ({ mutateAsync: updateMutate, isPending: false }),
}));

function fillRequired() {
  const fields: [string, string][] = [
    ["Nama", "SW-TEST"],
    ["IP atau hostname", "192.0.2.10"],
    ["SNMP community", "c"],
    ["User (read-only)", "tikman"],
    ["Password", "p"],
  ];
  for (const [label, value] of fields) {
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
  }
}

const saveButton = () =>
  screen.getByText("Simpan").closest("button") as HTMLButtonElement;

describe("switch protocols", () => {
  it("offers each vendor only the protocols its driver speaks", () => {
    expect(protocolsFor("huawei_ce")).toEqual(["ssh", "telnet"]);
    expect(protocolsFor("fiberhome")).toEqual(["ssh", "telnet"]);
    expect(protocolsFor("crs")).toEqual(["rest"]);
  });
});

describe("SwitchFormModal", () => {
  beforeEach(() => {
    testMutate.mockReset();
    createMutate.mockReset();
    updateMutate.mockReset();
  });

  it("asks for the vendor first and only lets Huawei CE be chosen", () => {
    render(<SwitchFormModal onClose={vi.fn()} />);
    const labels = Array.from(document.querySelectorAll("label[title]")).map(
      (l) => l.getAttribute("title"),
    );
    const vendor = (text: RegExp) =>
      screen.getByText(text).closest("label")?.querySelector("input");

    expect(labels[0]).toBe("Vendor");
    expect(vendor(/^Huawei CE$/)?.disabled).toBe(false);
    expect(vendor(/CRS.*segera/)?.disabled).toBe(true);
    expect(vendor(/FiberHome.*segera/)?.disabled).toBe(true);
    expect(screen.getByText("SSH")).toBeTruthy();
    expect(screen.getByText("Telnet")).toBeTruthy();
    expect(screen.queryByText("REST")).toBeNull();
  });

  it("warns that telnet is unencrypted and moves the port to 23", async () => {
    render(<SwitchFormModal onClose={vi.fn()} />);
    expect(screen.queryByText(TELNET_WARNING)).toBeNull();

    fireEvent.click(screen.getByText("Telnet"));

    expect(await screen.findByText(TELNET_WARNING)).toBeTruthy();
    expect((screen.getByLabelText("Port") as HTMLInputElement).value).toBe(
      "23",
    );
  });

  it("saves an SSH switch only once its key was confirmed, pinned to it", async () => {
    testMutate
      .mockResolvedValueOnce({
        kind: "unpinned",
        presentedFingerprint: "SHA256:abc",
      })
      .mockResolvedValueOnce({
        kind: "identified",
        model: "CE6850-48S4Q-EI",
        version: "V100R006C00",
      });
    createMutate.mockResolvedValue({});
    render(<SwitchFormModal onClose={vi.fn()} />);
    fillRequired();

    fireEvent.click(screen.getByText("Test koneksi"));
    expect(await screen.findByText(/SHA256:abc/)).toBeTruthy();
    expect(saveButton().disabled).toBe(true);
    expect(testMutate.mock.calls[0][0].hostKeyFingerprint).toBeUndefined();

    fireEvent.click(screen.getByText("Percayai kunci ini"));
    expect(await screen.findByText(/CE6850-48S4Q-EI/)).toBeTruthy();
    expect(testMutate.mock.calls[1][0].hostKeyFingerprint).toBe("SHA256:abc");
    fireEvent.click(saveButton());

    await waitFor(() => expect(createMutate).toHaveBeenCalled());
    expect(createMutate.mock.calls[0][0]).toMatchObject({
      vendor: "huawei_ce",
      protocol: "ssh",
      port: 22,
      hostKeyFingerprint: "SHA256:abc",
    });
  });

  it("saves a telnet switch after one passing test, with no key", async () => {
    testMutate.mockResolvedValue({
      kind: "identified",
      model: "CE6850-48S4Q-EI",
      version: "V100R006C00",
    });
    createMutate.mockResolvedValue({});
    render(<SwitchFormModal onClose={vi.fn()} />);
    fireEvent.click(screen.getByText("Telnet"));
    fillRequired();

    fireEvent.click(screen.getByText("Test koneksi"));
    await screen.findByText(/CE6850-48S4Q-EI/);
    fireEvent.click(saveButton());

    await waitFor(() => expect(createMutate).toHaveBeenCalled());
    expect(createMutate.mock.calls[0][0]).toMatchObject({ protocol: "telnet" });
    expect(createMutate.mock.calls[0][0].hostKeyFingerprint).toBeUndefined();
  });

  it("drops a confirmed key once the form changes", async () => {
    testMutate.mockResolvedValue({
      kind: "unpinned",
      presentedFingerprint: "SHA256:abc",
    });
    render(<SwitchFormModal onClose={vi.fn()} />);
    fillRequired();
    fireEvent.click(screen.getByText("Test koneksi"));
    await screen.findByText(/SHA256:abc/);

    fireEvent.change(screen.getByLabelText("IP atau hostname"), {
      target: { value: "192.0.2.11" },
    });

    expect(screen.queryByText(/SHA256:abc/)).toBeNull();
    expect(saveButton().disabled).toBe(true);
  });
});
