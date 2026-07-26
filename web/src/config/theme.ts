/**
 * Global Color Theme
 *
 * Inspired by GitHub's Primer dark mode design system.
 * Edit here and changes propagate everywhere via CSS custom properties.
 */

export const theme = {
  colors: {
    canvas: "#0d1117", // GitHub canvas.default - deepest layer
    surface: "#161b22", // GitHub canvas.subtle - sidebar, cards, panels
    surfaceAlt: "#1c2128", // slightly elevated surface
    surfaceHigh: "#21262d", // GitHub canvas.inset - hover rows, table alt
    surfaceHighest: "#2d333b", // popovers, dropdowns, tooltips

    primary: "#f0f6fc", // near-white - GitHub fg.on-emphasis - selected states, CTAs
    primaryHover: "#ffffff", // pure white on hover

    success: "#3fb950", // GitHub success.fg
    warning: "#d29922", // GitHub attention.fg
    danger: "#f85149", // GitHub danger.fg

    ink: "#e6edf3", // GitHub fg.default - primary text (cool white)
    inkDim: "#8b949e", // GitHub fg.muted - secondary / label text
    inkFaint: "#6e7681", // GitHub fg.subtle - placeholder / disabled

    rim: "#30363d", // GitHub border.default
    rimStrong: "#484f58", // GitHub border.muted - emphasis
  },
} as const;

export type ThemeColors = typeof theme.colors;

/** Injects theme colors as CSS custom properties on :root. Called once at startup. */
export function applyTheme() {
  const root = document.documentElement;
  const { colors } = theme;

  const map: Record<string, string> = {
    "--canvas": colors.canvas,
    "--surface": colors.surface,
    "--surface-alt": colors.surfaceAlt,
    "--surface-high": colors.surfaceHigh,
    "--surface-highest": colors.surfaceHighest,
    "--primary": colors.primary,
    "--primary-hover": colors.primaryHover,
    "--success": colors.success,
    "--warning": colors.warning,
    "--danger": colors.danger,
    "--ink": colors.ink,
    "--ink-dim": colors.inkDim,
    "--ink-faint": colors.inkFaint,
    "--rim": colors.rim,
    "--rim-strong": colors.rimStrong,
  };

  for (const [prop, value] of Object.entries(map)) {
    root.style.setProperty(prop, value);
  }
}
