import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { mikrotikRouter } from "@/test/mikrotikRouterFixture";
import { RouterFormModal } from "./RouterFormModal";

const testMutate = vi.fn();
const createMutate = vi.fn();
const updateMutate = vi.fn();

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useTestRouterConnection: () => ({
    mutateAsync: testMutate,
    isPending: false,
  }),
  useCreateMikrotikRouter: () => ({
    mutateAsync: createMutate,
    isPending: false,
  }),
  useUpdateMikrotikRouter: () => ({
    mutateAsync: updateMutate,
    isPending: false,
  }),
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
    updateMutate.mockReset();
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

  it("lets an admin turn on NetFlow with its sampling ratio when editing", async () => {
    updateMutate.mockResolvedValue({});
    render(<RouterFormModal router={mikrotikRouter()} onClose={vi.fn()} />);

    fireEvent.click(screen.getByLabelText("Kumpulkan NetFlow"));
    fireEvent.change(screen.getByLabelText("Rasio sampling (1:N)"), {
      target: { value: "512" },
    });
    fireEvent.click(screen.getByText("Simpan"));

    await waitFor(() => expect(updateMutate).toHaveBeenCalled());
    expect(updateMutate.mock.calls[0][0].data).toMatchObject({
      flowEnabled: true,
      flowSampling: 512,
    });
  });

  it("shows the rack position of the router being edited", () => {
    render(
      <RouterFormModal
        router={mikrotikRouter({ rackPosition: 2 })}
        onClose={vi.fn()}
      />,
    );

    expect(
      (screen.getByLabelText("Posisi di rak") as HTMLInputElement).value,
    ).toBe("2");
  });

  it("clears the rack position instead of sending null", async () => {
    updateMutate.mockResolvedValue({});
    render(
      <RouterFormModal
        router={mikrotikRouter({ rackPosition: 2 })}
        onClose={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText("Posisi di rak"), {
      target: { value: "" },
    });
    fireEvent.click(screen.getByText("Simpan"));

    await waitFor(() => expect(updateMutate).toHaveBeenCalled());
    const data = updateMutate.mock.calls[0][0].data;
    expect(data.clearRackPosition).toBe(true);
    expect("rackPosition" in data).toBe(false);
  });

  it("asks the backend to guess again when an admin's role goes back to Otomatis", async () => {
    updateMutate.mockResolvedValue({});
    render(
      <RouterFormModal
        router={mikrotikRouter({
          role: "border",
          roleSetByAdmin: true,
          roleGuess: "bras",
        })}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Tebakan TikMan: BRAS")).toBeTruthy();
    fireEvent.click(screen.getByLabelText("Otomatis"));
    fireEvent.click(screen.getByText("Simpan"));

    await waitFor(() => expect(updateMutate).toHaveBeenCalled());
    const data = updateMutate.mock.calls[0][0].data;
    expect(data.resetRole).toBe(true);
    expect("role" in data).toBe(false);
  });

  it("leaves the role alone when a router already on Otomatis is saved", async () => {
    updateMutate.mockResolvedValue({});
    render(
      <RouterFormModal
        router={mikrotikRouter({ roleSetByAdmin: false, roleGuess: "bras" })}
        onClose={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByText("Simpan"));

    await waitFor(() => expect(updateMutate).toHaveBeenCalled());
    const data = updateMutate.mock.calls[0][0].data;
    expect("resetRole" in data).toBe(false);
    expect("role" in data).toBe(false);
  });

  it("sends the role the admin chose", async () => {
    updateMutate.mockResolvedValue({});
    render(<RouterFormModal router={mikrotikRouter()} onClose={vi.fn()} />);

    fireEvent.click(screen.getByLabelText("BRAS"));
    fireEvent.click(screen.getByText("Simpan"));

    await waitFor(() => expect(updateMutate).toHaveBeenCalled());
    const data = updateMutate.mock.calls[0][0].data;
    expect(data.role).toBe("bras");
    expect("resetRole" in data).toBe(false);
  });

  it("says so when TikMan has no guess yet", () => {
    render(<RouterFormModal router={mikrotikRouter()} onClose={vi.fn()} />);
    expect(screen.getByText("Belum ditebak")).toBeTruthy();
  });

  it("starts a new router on Otomatis and sends no role for it", async () => {
    testMutate.mockResolvedValue({ boardName: "CCR2116", version: "7.24.2" });
    createMutate.mockResolvedValue({});
    render(<RouterFormModal onClose={vi.fn()} />);
    fireEvent.click(screen.getByText("API (tanpa enkripsi)"));
    fillRequired();
    fireEvent.click(screen.getByText("Uji koneksi"));
    await waitFor(() => expect(testMutate).toHaveBeenCalled());
    fireEvent.click(screen.getByText("Simpan"));

    await waitFor(() => expect(createMutate).toHaveBeenCalled());
    expect("role" in createMutate.mock.calls[0][0]).toBe(false);
  });

  it("does not offer NetFlow while adding a router", () => {
    render(<RouterFormModal onClose={vi.fn()} />);

    expect(screen.queryByLabelText("Kumpulkan NetFlow")).toBeNull();
  });
});
