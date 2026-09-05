import { Grid } from "antd";
import { version } from "../../../../package.json";
import { FOOTER_HEIGHT } from "./layoutPadding";

const AUTHOR = "Rohadi M Raja";

/**
 * AppFooter is the credit line under every admin page.
 *
 * Its height is fixed to FOOTER_HEIGHT rather than left to the padding,
 * because fullHeightPage subtracts that number to keep the CS inbox from
 * scrolling — an approximation there would put the scrollbar back.
 *
 * The text stays on one line for the same reason, which is why it shrinks a
 * point on a phone: at 12px the full credit needs about 300px, and a 320px
 * screen would wrap it into a second line the reserved height has no room for.
 */
export function AppFooter() {
  const screens = Grid.useBreakpoint();

  return (
    <div
      style={{
        height: FOOTER_HEIGHT,
        boxSizing: "border-box",
        borderTop: "1px solid #27272a",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        gap: 6,
        fontSize: screens.xs ? 11 : 12,
        color: "#71717a",
        whiteSpace: "nowrap",
      }}
    >
      <span>TikMan © {new Date().getFullYear()}</span>
      <span aria-hidden>·</span>
      <span>Dibuat oleh {AUTHOR}</span>
      <span aria-hidden>·</span>
      <span>v{version}</span>
    </div>
  );
}
