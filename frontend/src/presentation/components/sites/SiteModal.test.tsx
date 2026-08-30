import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SiteModal } from "./SiteModal";

describe("SiteModal", () => {
  it("saves a site typed with coordinates by hand", async () => {
    // Places does not know a POP down a gang or a tower in a field, which is a
    // large share of these sites. Manual entry is the path that always works.
    const onSubmit = vi.fn();
    render(
      <SiteModal open onClose={vi.fn()} onSubmit={onSubmit} loading={false} />,
    );

    await userEvent.type(screen.getByLabelText("Site Name"), "Gudang");
    await userEvent.type(screen.getByLabelText("Latitude"), "-6.4025");
    await userEvent.type(screen.getByLabelText("Longitude"), "106.7942");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        name: "Gudang",
        latitude: -6.4025,
        longitude: 106.7942,
      }),
    );
  });

  it("saves a site with no coordinates at all", async () => {
    const onSubmit = vi.fn();
    render(
      <SiteModal open onClose={vi.fn()} onSubmit={onSubmit} loading={false} />,
    );

    await userEvent.type(screen.getByLabelText("Site Name"), "Gudang");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ name: "Gudang" }),
    );
    expect(onSubmit.mock.calls[0][0].latitude).toBeUndefined();
  });

  it("refuses half a coordinate and says why", async () => {
    const onSubmit = vi.fn();
    render(
      <SiteModal open onClose={vi.fn()} onSubmit={onSubmit} loading={false} />,
    );

    await userEvent.type(screen.getByLabelText("Site Name"), "Gudang");
    await userEvent.type(screen.getByLabelText("Latitude"), "-6.4025");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(await screen.findByText(/together/i)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
