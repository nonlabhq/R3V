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
  { name: "Geist", value: `"Geist Variable", "Noto Sans TC", "Segoe UI", system-ui, sans-serif` },
  { name: "Segoe UI", value: `"Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif` },
  { name: "System", value: `system-ui, sans-serif` },
  { name: "Inter", value: `"Inter", "Segoe UI", system-ui, sans-serif` },
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

// The earlier look (before the R3V brand), under the other looks too.
const classicVars: Record<string, string> = {
  "--bg-sunken": "#141518",
  "--bg": "#18191d",
  "--panel": "#202227",
  "--panel-2": "#272a30",
  "--panel-3": "#2a2c31",
  "--hover-soft": "#2e3138",
  "--hover": "#33363d",
  "--hover-strong": "#3a3d45",
  "--line-soft": "#25272c",
  "--line": "#34373e",
  "--line-strong": "#4a4e57",
  "--text": "#e6e7ea",
  "--muted": "#9a9ea8",
  "--faint": "#6c707a",
  "--dim": "#5b606b",
  "--switch-knob": "#d8dae0",
  "--accent": "#3fc3a9",
  "--accent-hover": "#52d4ba",
  "--accent-ink": "#0d2b25",
  "--accent-text": "#d9f3ec",
  "--accent-bg": "#1f3b35",
  "--accent-line": "#2c5a4e",
  "--accent-soft": "rgba(63, 195, 169, .18)",
  "--warn": "#e8b04b",
  "--warn-bg": "#3a2f1a",
  "--warn-line": "#5a4623",
  "--warn-strong": "#6b5426",
  "--warn-text": "#f0d9a8",
  "--warn-ink": "#1b1407",
  "--warn-soft": "rgba(232, 176, 75, .16)",
  "--danger": "#e5675f",
  "--danger-bg": "#3a1f1f",
  "--danger-line": "#6a3030",
  "--danger-text": "#f3c0bc",
  "--danger-soft": "rgba(229, 103, 95, .16)",
  "--info": "#6ab0f3",
  "--info-bg": "#1d2c38",
  "--info-line": "#2c4557",
  "--info-chip": "#2b3a45",
  "--info-text": "#9fd0f5",
  "--past-bg": "#2a2536",
  "--past-line": "#463c5c",
  "--add": "#6fcf7f",
  "--del": "#e5675f",
  "--mod": "#6ab0f3",
  "--add-soft": "rgba(111, 207, 127, .16)",
  "--del-soft": "rgba(229, 103, 95, .16)",
  "--mod-soft": "rgba(106, 176, 243, .16)",
  "--lane-0": "#3fc3a9",
  "--lane-1": "#6ab0f3",
  "--lane-2": "#c792ea",
  "--lane-3": "#e8b04b",
  "--lane-4": "#f07178",
  "--font-sans": "\"Inter\", \"Segoe UI\", system-ui, sans-serif",
  "--radius-xs": "3px",
  "--radius-sm": "4px",
  "--radius": "6px",
  "--radius-lg": "8px",
  "--radius-xl": "12px",
  "--shadow-pop": "0 12px 30px rgba(0, 0, 0, .45)",
  "--shadow-dialog": "0 20px 60px rgba(0, 0, 0, .5)",
  "--scrim": "rgba(8, 9, 11, .62)",
  "--scrim-strong": "rgba(8, 9, 11, .82)",
  "--surface-menu": "var(--panel-2)",
  "--surface-dialog": "var(--panel)",
  "--surface-filter": "none",
  "--ok": "#3fc3a9",
  "--font-brand": "var(--font-sans)",
  "--radius-card": "12px",
  "--surface-float": "var(--panel)",
  "--float-filter": "none",
  "--shadow-float": "var(--shadow-dialog)",
  "--shadow-card": "none",
  "--dot-grid": "transparent",
};

const r3v: Knobs = {
  density: 1, radius: 8, border: 1, line: "#242424", accent: "#ff4a1c",
  surfaceAlpha: 0.92, blur: 18, scrimBlur: 0, shadow: 1, font: fonts[0].value,
};
const base: Knobs = {
  density: 1, radius: 6, border: 1, line: "#34373e", accent: "#3fc3a9",
  surfaceAlpha: 1, blur: 0, scrimBlur: 0, shadow: 1, font: fonts[3].value,
};

export const themes: Theme[] = [
  { id: "r3v", name: "R3V", note: "The R3V look: dark Bento, brand orange", knobs: r3v, vars: {} },
  { id: "classic", name: "Classic", note: "R3V before the brand: teal on grey", knobs: base, vars: classicVars },
  {
    id: "studio", name: "Studio", note: "A DAW's tool look: tight, square, neutral greys",
    knobs: { ...base, density: 0.85, radius: 3, line: "#3a3a3a", accent: "#f2a33a", shadow: 0.6 },
    vars: {
      ...classicVars,
      "--bg-sunken": "#161616", "--bg": "#1b1b1b", "--panel": "#222222", "--panel-2": "#2a2a2a",
      "--panel-3": "#2e2e2e", "--hover-soft": "#303030", "--hover": "#353535", "--hover-strong": "#3c3c3c",
      "--line-soft": "#262626", "--line": "#3a3a3a", "--line-strong": "#4d4d4d",
      "--text": "#e4e4e4", "--muted": "#a0a0a0", "--faint": "#707070",
      ...accentVars("#f2a33a"),
    },
  },
  {
    id: "soft", name: "Soft", note: "A modern app: roomy, round, gentle contrast",
    knobs: { ...base, density: 1.15, radius: 10, line: "#2e3140", accent: "#8b9cff", shadow: 1.3, font: fonts[1].value },
    vars: {
      ...classicVars,
      "--bg-sunken": "#15161c", "--bg": "#1a1b22", "--panel": "#20222b", "--panel-2": "#272a35",
      "--panel-3": "#2b2e3a", "--hover-soft": "#2d3040", "--hover": "#323647", "--hover-strong": "#393d50",
      "--line-soft": "#23252f", "--line": "#2e3140", "--line-strong": "#464a5e",
      "--text": "#e8e9f0", "--muted": "#a0a4b8", "--faint": "#6e7286",
      ...accentVars("#8b9cff"),
    },
  },
  {
    id: "glass", name: "Glass", note: "See-through menus and dialogs over a deep backdrop",
    knobs: { ...base, radius: 12, line: "#2c303b", accent: "#5cd3e8", surfaceAlpha: 0.6, blur: 24, scrimBlur: 6, shadow: 1.2 },
    vars: {
      ...classicVars,
      "--bg-sunken": "#0e0f13", "--bg": "#13141a", "--panel": "#1a1c23", "--panel-2": "#22252e",
      "--panel-3": "#262a34", "--hover-soft": "#292d38", "--hover": "#2e3340", "--hover-strong": "#353a48",
      "--line-soft": "#1d2027", "--line": "#2c303b", "--line-strong": "#434957",
      "--scrim": "rgba(6, 7, 10, .45)",
      "--surface-menu": "color-mix(in srgb, var(--panel-2) 60%, transparent)",
      "--surface-dialog": "color-mix(in srgb, var(--panel) 60%, transparent)",
      "--surface-filter": "blur(24px) saturate(1.4)", "--scrim-filter": "blur(6px)",
      "--surface-float": "color-mix(in srgb, var(--panel) 70%, transparent)", "--float-filter": "blur(24px) saturate(1.4)",
      ...accentVars("#5cd3e8"),
    },
  },
];

/** The tokens a theme sets, with the knobs moved in the lab on top (a knob
 *  left where the theme has it changes nothing). */
export function tokens(theme: Theme, k: Knobs): Record<string, string> {
  const px = (n: number) => `${Math.round(n * 2) / 2}px`;
  const t = theme.knobs;
  const v: Record<string, string> = { ...theme.vars };
  if (k.density !== t.density) v["--density"] = String(k.density);
  if (k.radius !== t.radius) Object.assign(v, {
    "--radius-xs": px(k.radius / 2), "--radius-sm": px((k.radius * 2) / 3), "--radius": px(k.radius),
    "--radius-lg": px((k.radius * 4) / 3), "--radius-xl": px(k.radius * 2), "--radius-card": px(k.radius * 2.75),
  });
  if (k.border !== t.border) v["--border-width"] = `${k.border}px`;
  if (k.surfaceAlpha !== t.surfaceAlpha) {
    const a = Math.round(k.surfaceAlpha * 100);
    v["--surface-menu"] = k.surfaceAlpha < 1 ? `color-mix(in srgb, var(--panel-2) ${a}%, transparent)` : "var(--panel-2)";
    v["--surface-dialog"] = k.surfaceAlpha < 1 ? `color-mix(in srgb, var(--panel) ${a}%, transparent)` : "var(--panel)";
    v["--surface-float"] = k.surfaceAlpha < 1 ? `color-mix(in srgb, var(--panel-2) ${a}%, transparent)` : "var(--panel-2)";
  }
  if (k.blur !== t.blur) v["--surface-filter"] = v["--float-filter"] = k.blur > 0 ? `blur(${k.blur}px) saturate(1.2)` : "none";
  if (k.scrimBlur !== t.scrimBlur) v["--scrim-filter"] = k.scrimBlur > 0 ? `blur(${k.scrimBlur}px)` : "none";
  if (k.shadow !== t.shadow) {
    v["--shadow-pop"] = `0 12px 30px rgba(0, 0, 0, ${+(0.45 * k.shadow).toFixed(2)})`;
    v["--shadow-dialog"] = v["--shadow-float"] = `0 20px 60px rgba(0, 0, 0, ${+(0.5 * k.shadow).toFixed(2)})`;
  }
  if (k.font !== t.font) v["--font-sans"] = k.font;
  if (k.line !== t.line) v["--line"] = k.line;
  if (k.accent !== t.accent) Object.assign(v, accentVars(k.accent));
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
    return s && themes.some((t) => t.id === s.theme) ? { ...s, knobs: { ...(themes.find((t) => t.id === s.theme)!.knobs), ...s.knobs } } : null;
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
