// The app's look, picked in R3V's settings and kept on this computer:
// Flat (panels side by side, the default) or Cards (the earlier look:
// cards floating on the window). It is data-look on <html>; tokens.css
// gives each its values, so the components are the same in both.

export const looks = ["flat", "cards"] as const;
export type Look = (typeof looks)[number];

const KEY = "r3v.look";
const DEFAULT: Look = "flat"; // for new users, and those who never chose

function saved(): Look {
  try {
    const v = localStorage.getItem(KEY);
    return looks.includes(v as Look) ? (v as Look) : DEFAULT;
  } catch {
    return DEFAULT;
  }
}

const state = $state<{ look: Look }>({ look: DEFAULT });

/** The look in use. */
export function look(): Look {
  return state.look;
}

function apply(l: Look) {
  state.look = l;
  document.documentElement.dataset.look = l;
}

/** A length token's value now, in px (0 when it isn't one), for the few
 * places that place things in script; read again when the look changes.
 * (Components never ask which look is in use: they read its tokens.) */
export function tokenPx(name: string): number {
  void state.look;
  if (typeof getComputedStyle !== "function") return 0;
  return parseFloat(getComputedStyle(document.documentElement).getPropertyValue(name)) || 0;
}

/** Uses a look and remembers it. */
export function setLook(l: Look) {
  try {
    localStorage.setItem(KEY, l);
  } catch {
    /* not remembered: still used now */
  }
  apply(l);
}

/** Puts the remembered look on the page (before the app shows). */
export function startLook() {
  apply(saved());
}
