// Semantic colors for status surfaces. The antd theme tokens cover text, borders
// and containers, but the dashboard also needs tinted backgrounds per status,
// which have no token equivalent.
export const colors = {
  surface: "#18181b",
  border: "#27272a",
  textPrimary: "#ffffff",
  textSecondary: "#a1a1aa",
  textMuted: "#71717a",
  textBody: "#e5e5e5",

  success: "#3ecf8e",
  danger: "#f87171",
  warning: "#fbbf24",
  neutral: "#a1a1aa",
} as const;

// Status surfaces are a low-alpha wash of the accent over the card, not a solid
// fill. Solid blocks (#14532d, #450a0a) read as slabs on a near-black canvas and
// give every tile the same visual weight regardless of whether it needs action.
export const statusSurfaces = {
  success: {
    bg: "rgba(62, 207, 142, 0.08)",
    border: "rgba(62, 207, 142, 0.28)",
    accent: colors.success,
    hint: "rgba(62, 207, 142, 0.72)",
  },
  danger: {
    bg: "rgba(248, 113, 113, 0.09)",
    border: "rgba(248, 113, 113, 0.32)",
    accent: colors.danger,
    hint: "rgba(248, 113, 113, 0.72)",
  },
  warning: {
    bg: "rgba(251, 191, 36, 0.09)",
    border: "rgba(251, 191, 36, 0.30)",
    accent: colors.warning,
    hint: "rgba(251, 191, 36, 0.72)",
  },
  neutral: {
    bg: "rgba(161, 161, 170, 0.05)",
    border: colors.border,
    accent: colors.textSecondary,
    hint: colors.textMuted,
  },
  // Used when a tile has nothing to report: same geometry, no colour claim.
  quiet: {
    bg: "transparent",
    border: colors.border,
    accent: colors.textSecondary,
    hint: colors.textMuted,
  },
} as const;

export type StatusSurface = keyof typeof statusSurfaces;
