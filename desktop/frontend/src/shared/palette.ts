// The palette (tokens.css --palette-b1..b12, the R3V VI): the colours
// people pick for themselves and their projects, and the branches'
// colours. A team stores a colour's number ("b3"), never its value or name,
// so the colours can be tuned: add numbers, never reuse one. One this build
// doesn't know shows as no pick. In order: each is far from the one before.

export const palette = ["b1", "b2", "b3", "b4", "b5", "b6", "b7", "b8", "b9", "b10", "b11", "b12"] as const;

export type PaletteName = (typeof palette)[number];

const known = new Set<string>(palette);

function hash(s: string): number {
  return [...s].reduce((h, c) => (h * 31 + c.codePointAt(0)!) >>> 0, 7);
}

/** The colour picked from seed when none was chosen: the same everywhere. */
export function pickFor(seed: string): PaletteName {
  return palette[hash(seed) % palette.length];
}

/** A project's colour: the chosen one, else one picked from seed (its id,
 *  else its name), the same everywhere. */
export function projectColor(seed: string, chosen = ""): string {
  return colorOf(chosen, seed);
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
