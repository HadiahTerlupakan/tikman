import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { version } from "../../../../package.json";
import { AppFooter } from "./AppFooter";
import { FOOTER_HEIGHT } from "./layoutPadding";

describe("AppFooter", () => {
  it("credits the author", () => {
    render(<AppFooter />);

    expect(screen.getByText(/Rohadi M Raja/)).toBeInTheDocument();
  });

  // Read from package.json rather than written here, so bumping the version in
  // one place moves the footer with it.
  it("shows the version the app was built at", () => {
    render(<AppFooter />);

    expect(screen.getByText(new RegExp(`v${version}`))).toBeInTheDocument();
  });

  // fullHeightPage subtracts FOOTER_HEIGHT to keep the CS inbox from scrolling.
  // That subtraction is a lie unless the footer really occupies it, so the
  // height is asserted rather than left to whatever the padding measures.
  it("occupies exactly the height the page arithmetic reserves for it", () => {
    render(<AppFooter />);

    expect(screen.getByRole("contentinfo")).toHaveStyle({
      height: `${FOOTER_HEIGHT}px`,
    });
  });
});
