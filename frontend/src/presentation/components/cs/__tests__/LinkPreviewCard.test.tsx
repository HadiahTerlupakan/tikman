import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LinkPreviewCard } from "../LinkPreviewCard";

const preview = {
  url: "https://example.com/a/b",
  title: "Judul",
  description: "Deskripsi",
};

describe("LinkPreviewCard", () => {
  it("shows the title, the description and the site", () => {
    render(<LinkPreviewCard preview={preview} />);

    expect(screen.getByText("Judul")).toBeInTheDocument();
    expect(screen.getByText("Deskripsi")).toBeInTheDocument();
    expect(screen.getByText("example.com")).toBeInTheDocument();
  });

  // In the composer the CS can decide not to send the card.
  it("offers a way out when one is given", async () => {
    const onDismiss = vi.fn();
    render(<LinkPreviewCard preview={preview} onDismiss={onDismiss} />);

    await userEvent.click(screen.getByRole("button", { name: /sembunyikan/i }));

    expect(onDismiss).toHaveBeenCalledOnce();
  });

  // In a sent message there is nothing to dismiss — the card is part of what
  // the customer already received.
  it("offers none in a bubble", () => {
    render(<LinkPreviewCard preview={preview} />);

    expect(screen.queryByRole("button")).toBeNull();
  });
});
