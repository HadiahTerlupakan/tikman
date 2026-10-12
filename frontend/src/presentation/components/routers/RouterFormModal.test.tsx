import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { RouterFormModal } from "./RouterFormModal";

const testMutate = vi.fn();
const createMutate = vi.fn();

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useTestRouterConnection: () => ({
    mutateAsync: testMutate,
    isPending: false,
  }),
  useCreateMikrotikRouter: () => ({
    mutateAsync: createMutate,
    isPending: false,
  }),
  useUpdateMikrotikRouter: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

function portInput(): HTMLInputElement {
  return screen.getByLabelText("Port API") as HTMLInputElement;
}

function fillRequired() {
  fireEvent.change(screen.getByLabelText("Nama"), {
    target: { value: "Border" },
  });
  fireEvent.change(screen.getByLabelText("IP atau hostname"), {
    target: { value: "192.0.2.1" },
  });
  fireEvent.change(screen.getByLabelText("SNMP community"), {
    target: { value: "c" },
  });
  fireEvent.change(screen.getByLabelText("User API (read-only)"), {
    target: { value: "tikman" },
  });
  fireEvent.change(screen.getByLabelText("Password API"), {
    target: { value: "p" },
  });
}

describe("RouterFormModal protocol", () => {
  beforeEach(() => {
    testMutate.mockReset();
    createMutate.mockReset();
  });

  it("moves the port to the protocol's default", async () => {
    render(<RouterFormModal onClose={vi.fn()} />);
    expect(portInput().value).toBe("8443");

    fireEvent.click(screen.getByText("API (tanpa enkripsi)"));

    await waitFor(() => expect(portInput().value).toBe("8728"));
    expect(screen.getByText("Password dikirim tanpa enkripsi")).toBeTruthy();
  });

  it("keeps a port the operator typed", async () => {
    render(<RouterFormModal onClose={vi.fn()} />);
    fireEvent.change(portInput(), { target: { value: "65489" } });

    fireEvent.click(screen.getByText("API (tanpa enkripsi)"));

    await waitFor(() => expect(portInput().value).toBe("65489"));
  });

  it("saves a plain-API router after one passing test, without a fingerprint", async () => {
    testMutate.mockResolvedValue({ boardName: "CCR2116", version: "7.24.2" });
    createMutate.mockResolvedValue({});
    render(<RouterFormModal onClose={vi.fn()} />);
    fireEvent.click(screen.getByText("API (tanpa enkripsi)"));
    fillRequired();

    fireEvent.click(screen.getByText("Uji koneksi"));
    await screen.findByText(/CCR2116/);
    fireEvent.click(screen.getByText("Simpan"));

    await waitFor(() => expect(createMutate).toHaveBeenCalled());
    expect(createMutate.mock.calls[0][0]).toMatchObject({ apiProtocol: "api" });
    expect(createMutate.mock.calls[0][0].tlsFingerprint).toBeUndefined();
  });
});
