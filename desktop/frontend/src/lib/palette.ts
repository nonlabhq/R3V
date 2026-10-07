// The palette (tokens.css --palette-*): the colours people pick for
// themselves and their projects, and the branches' colours. The names are
// what a team stores (a member's or a project's colour): add names, never
// rename or remove one. A name this build doesn't know shows as no pick.

export const palette = ["orange", "amber", "lime", "green", "teal", "cyan", "blue", "indigo", "violet", "pink",
  "red", "sand", "slate"] as const;

export type PaletteName = (typeof palette)[number];

const known = new Set<string>(palette);

function hash(s: string): number {
  return [...s].reduce((h, c) => (h * 31 + c.codePointAt(0)!) >>> 0, 7);
}

/** The colour picked from seed when none was chosen: the same everywhere. */
export function pickFor(seed: string): PaletteName {
  return palette[hash(seed) % palette.length];
}

/** A project's colour when none was chosen: as before the palette, one of
 *  lanes 1-4 picked from its name (lane 0 is the main branch's). */
export function projectColor(name: string, chosen = ""): string {
  if (known.has(chosen)) return chosen;
  return ["violet", "teal", "blue", "pink"][hash(name) % 4];
}

/** The colour to show: the chosen one if it is the palette's, else seed's. */
export function colorOf(chosen: string, seed: string): string {
  return known.has(chosen) ? chosen : pickFor(seed);
}

/** The CSS colour of a palette name. */
export const cssColor = (name: string) => `var(--palette-${name})`;

/** Another colour than now, at random (the "random" button). */
export function randomColor(now = ""): PaletteName {
  const others = palette.filter((c) => c !== now);
  return others[Math.floor(Math.random() * others.length)];
}

export const initial = (name: string) => ([...name.trim()][0] ?? "?").toUpperCase();
