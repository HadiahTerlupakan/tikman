import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { MessageText } from "../MessageText";

describe("MessageText", () => {
  it("leaves a message without a link as plain text", () => {
    render(<MessageText body="halo pak, sudah dicek ya" />);

    expect(screen.getByText(/halo pak, sudah dicek ya/)).toBeInTheDocument();
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("makes a link clickable and keeps the words around it", () => {
    render(<MessageText body="silakan buka https://example.com ya pak" />);

    const link = screen.getByRole("link", { name: "https://example.com" });
    expect(link).toHaveAttribute("href", "https://example.com");
    expect(screen.getByText(/silakan buka/)).toBeInTheDocument();
    expect(screen.getByText(/ya pak/)).toBeInTheDocument();
  });

  it("links every address in a message, not only the first", () => {
    render(
      <MessageText body="a https://one.example b https://two.example c" />,
    );

    expect(screen.getAllByRole("link")).toHaveLength(2);
  });

  // Trailing punctuation belongs to the sentence, not the address — a link
  // ending in a full stop 404s.
  it("leaves sentence punctuation out of the address", () => {
    render(<MessageText body="cek https://example.com/a." />);

    expect(screen.getByRole("link")).toHaveAttribute(
      "href",
      "https://example.com/a",
    );
  });

  // Without noopener the page we open can reach back through window.opener
  // and navigate this tab — a CS clicking a customer's link must not hand
  // the inbox to it.
  it("opens links in a new tab with no handle back to the inbox", () => {
    render(<MessageText body="https://example.com" />);

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link.getAttribute("rel")).toContain("noopener");
    expect(link.getAttribute("rel")).toContain("noreferrer");
  });

  // Only web addresses. A body containing javascript: or data: must stay text.
  it("refuses to link anything that is not http or https", () => {
    render(<MessageText body="javascript:alert(1) dan data:text/html,x" />);

    expect(screen.queryByRole("link")).toBeNull();
  });
});
