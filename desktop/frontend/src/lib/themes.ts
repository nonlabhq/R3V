// Looks to try (Nightly only, with the Style lab: Ctrl+Alt+L). A theme is a
// set of token values over tokens.css, plus the lab's knobs; the knobs
// write tokens too, so a look tried in the lab can be copied as CSS and
// made the default in tokens.css.

export type Knobs = {
  density: number;      // scales every --sp-*
  radius: number;       // px of --radius; the other corners follow
  border: number;       // px of hairlines
  line: string;         // hairline colour
  accent: string;
  surfaceAlpha: number; // menus and dialogs: 1 solid, less see-through
  blur: number;         // px of blur behind see-through surfaces
  scrimBlur: number;    // px of blur behind dialogs
  shadow: number;       // shadow strength, 1 as designed
  font: string;
};

export type Theme = { id: string; name: string; note: string; knobs: Knobs; vars: Record<string, string> };

export const fonts = [
  { name: "Inter", value: `"Inter", "Segoe UI", system-ui, sans-serif` },
  { name: "Segoe UI", value: `"Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif` },
  { name: "System", value: `system-ui, sans-serif` },
];

/** The accent and the tones made from it (as tokens.css names them). */
export function accentVars(a: string): Record<string, string> {
  return {
    "--accent": a,
    "--accent-hover": `color-mix(in srgb, ${a}, white 15%)`,
    "--accent-ink": luminance(a) > 0.45 ? `color-mix(in srgb, ${a} 20%, black)` : "#ffffff",
    "--accent-text": `color-mix(in srgb, ${a} 25%, white)`,
    "--accent-bg": `color-mix(in srgb, ${a} 20%, var(--bg))`,
    "--accent-line": `color-mix(in srgb, ${a} 38%, var(--bg))`,
    "--accent-soft": `color-mix(in srgb, ${a} 18%, transparent)`,
    "--lane-0": a,
  };
}

function luminance(hex: string): number {
  const n = parseInt(hex.replace("#", ""), 16);
  const c = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
}

const base: Knobs = {
  density: 1, radius: 6, border: 1, line: "#34373e", accent: "#3fc3a9",
  surfaceAlpha: 1, blur: 0, scrimBlur: 0, shadow: 1, font: fonts[0].value,
};

export const themes: Theme[] = [
  { id: "current", name: "Current", note: "R3V as it is", knobs: base, vars: {} },
  {
    id: "studio", name: "Studio", note: "A DAW's tool look: tight, square, neutral greys",
    knobs: { ...base, density: 0.85, radius: 3, line: "#3a3a3a", accent: "#f2a33a", shadow: 0.6 },
    vars: {
      "--bg-sunken": "#161616", "--bg": "#1b1b1b", "--panel": "#222222", "--panel-2": "#2a2a2a",
      "--panel-3": "#2e2e2e", "--hover-soft": "#303030", "--hover": "#353535", "--hover-strong": "#3c3c3c",
      "--line-soft": "#262626", "--line-strong": "#4d4d4d",
      "--text": "#e4e4e4", "--muted": "#a0a0a0", "--faint": "#707070",
      ...accentVars("#f2a33a"),
    },
  },
  {
    id: "soft", name: "Soft", note: "A modern app: roomy, round, gentle contrast",
    knobs: { ...base, density: 1.15, radius: 10, line: "#2e3140", accent: "#8b9cff", shadow: 1.3,
      font: fonts[1].value },
    vars: {
      "--bg-sunken": "#15161c", "--bg": "#1a1b22", "--panel": "#20222b", "--panel-2": "#272a35",
      "--panel-3": "#2b2e3a", "--hover-soft": "#2d3040", "--hover": "#323647", "--hover-strong": "#393d50",
      "--line-soft": "#23252f", "--line-strong": "#464a5e",
      "--text": "#e8e9f0", "--muted": "#a0a4b8", "--faint": "#6e7286",
      ...accentVars("#8b9cff"),
    },
  },
  {
    id: "glass", name: "Glass", note: "See-through menus and dialogs over a deep backdrop",
    knobs: { ...base, radius: 12, line: "#2c303b", accent: "#5cd3e8", surfaceAlpha: 0.6, blur: 24,
      scrimBlur: 6, shadow: 1.2 },
    vars: {
      "--bg-sunken": "#0e0f13", "--bg": "#13141a", "--panel": "#1a1c23", "--panel-2": "#22252e",
      "--panel-3": "#262a34", "--hover-soft": "#292d38", "--hover": "#2e3340", "--hover-strong": "#353a48",
      "--line-soft": "#1d2027", "--line-strong": "#434957",
      "--scrim": "rgba(6, 7, 10, .45)",
      ...accentVars("#5cd3e8"),
    },
  },
];

/** The tokens a theme sets, with the knobs on top. */
export function tokens(theme: Theme, k: Knobs): Record<string, string> {
  const px = (n: number) => `${Math.round(n * 2) / 2}px`;
  const v: Record<string, string> = {
    ...theme.vars,
    "--density": String(k.density),
    "--radius-xs": px(k.radius / 2), "--radius-sm": px((k.radius * 2) / 3), "--radius": px(k.radius),
    "--radius-lg": px((k.radius * 4) / 3), "--radius-xl": px(k.radius * 2),
    "--border-width": `${k.border}px`,
    "--surface-menu": k.surfaceAlpha < 1 ? `color-mix(in srgb, var(--panel-2) ${Math.round(k.surfaceAlpha * 100)}%, transparent)` : "var(--panel-2)",
    "--surface-dialog": k.surfaceAlpha < 1 ? `color-mix(in srgb, var(--panel) ${Math.round(k.surfaceAlpha * 100)}%, transparent)` : "var(--panel)",
    "--surface-filter": k.blur > 0 ? `blur(${k.blur}px) saturate(1.4)` : "none",
    "--scrim-filter": k.scrimBlur > 0 ? `blur(${k.scrimBlur}px)` : "none",
    "--shadow-pop": `0 12px 30px rgba(0, 0, 0, ${+(0.45 * k.shadow).toFixed(2)})`,
    "--shadow-dialog": `0 20px 60px rgba(0, 0, 0, ${+(0.5 * k.shadow).toFixed(2)})`,
    "--font-sans": k.font,
  };
  // Colours only when changed in the lab (the theme already set its own).
  if (k.line !== theme.knobs.line) v["--line"] = k.line;
  else if (theme.id !== "current") v["--line"] = theme.knobs.line;
  if (k.accent !== theme.knobs.accent) Object.assign(v, accentVars(k.accent));
  return v;
}

/** The tokens as CSS, to paste into tokens.css. */
export function css(id: string, v: Record<string, string>): string {
  return `[data-theme="${id}"] {\n${Object.entries(v).map(([k, x]) => `  ${k}: ${x};`).join("\n")}\n}\n`;
}

const KEY = "r3v.styleLab";
export type Saved = { theme: string; knobs: Knobs; open: boolean };

export function load(): Saved | null {
  try {
    const s = JSON.parse(localStorage.getItem(KEY) ?? "null");
    return s && themes.some((t) => t.id === s.theme) ? { ...s, knobs: { ...base, ...s.knobs } } : null;
  } catch { return null; }
}

export function save(s: Saved) {
  try { localStorage.setItem(KEY, JSON.stringify(s)); } catch { /* not kept */ }
}

let applied: string[] = [];
/** Puts the tokens on the page (over tokens.css), replacing the last ones. */
export function apply(v: Record<string, string>) {
  const root = document.documentElement.style;
  for (const k of applied) root.removeProperty(k);
  for (const [k, x] of Object.entries(v)) root.setProperty(k, x);
  applied = Object.keys(v);
}
