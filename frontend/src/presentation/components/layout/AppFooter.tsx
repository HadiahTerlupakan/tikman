import { Grid } from "antd";
import { version } from "../../../../package.json";
import { FOOTER_HEIGHT } from "./layoutPadding";

const AUTHOR = "Rohadi M Raja";

const DIM = "#52525b";
const BRIGHT = "#a1a1aa";
const RULE = "#27272a";

function Dot() {
  return (
    <span aria-hidden style={{ color: "#3f3f46" }}>
      ·
    </span>
  );
}

/**
 * AppFooter is the credit line under every admin page.
 *
 * Its height is fixed to FOOTER_HEIGHT rather than left to the padding,
 * because fullHeightPage subtracts that number to keep the CS inbox from
 * scrolling — an approximation there would put the scrollbar back.
 *
 * The rule above it fades out at both ends. Nothing else in this UI draws a
 * bare line: every other boundary is the edge of a card, so an edge-to-edge
 * rule on the grid background read as something stuck on afterwards.
 *
 * The text stays on one line, which is why it shrinks a point on a phone: at
 * 12px the full credit needs about 300px, and a 320px screen would wrap it
 * into a second line the reserved height has no room for.
 */
export function AppFooter() {
  const screens = Grid.useBreakpoint();

  return (
    <footer
      style={{
        height: FOOTER_HEIGHT,
        boxSizing: "border-box",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <div
        style={{
          height: 1,
          background: `linear-gradient(90deg, transparent, ${RULE} 15%, ${RULE} 85%, transparent)`,
        }}
      />
      <div
        style={{
          flex: 1,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          gap: 8,
          fontSize: screens.xs ? 11 : 12,
          color: DIM,
          whiteSpace: "nowrap",
        }}
      >
        <span>TikMan © {new Date().getFullYear()}</span>
        <Dot />
        <span>
          Dibuat oleh <span style={{ color: BRIGHT }}>{AUTHOR}</span>
        </span>
        <Dot />
        <span>v{version}</span>
      </div>
    </footer>
  );
}
