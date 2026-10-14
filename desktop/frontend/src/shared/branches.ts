import { palette, pickFor } from "./palette";

// A branch as people see it: its name (what they called it, else its key)
// and its colour, a lane of the history graph (tokens.css --lane-N): 0 for
// the main branch (the brand's orange, its alone), else the palette's
// colour it was given, else one picked from its key (the same everywhere).

export type BranchLook = { name: string; label?: string; color?: string };

export const MAIN = "main";

/** What branch key is called. */
export function branchLabel(branches: BranchLook[] | undefined, key: string): string {
  return branches?.find((b) => b.name === key)?.label || key;
}

/** The lane colour (index of --lane-N) of branch key. */
export function branchLane(branches: BranchLook[] | undefined, key: string): number {
  if (key === MAIN) return 0;
  const chosen = palette.indexOf(branches?.find((b) => b.name === key)?.color as never);
  return (chosen >= 0 ? chosen : palette.indexOf(pickFor(key))) + 1;
}

/** The palette colour of branch key ("" for the main branch's). */
export function branchColor(branches: BranchLook[] | undefined, key: string): string {
  const lane = branchLane(branches, key);
  return lane ? palette[lane - 1] : "";
}

/** A colour for a new branch: the first of the palette no branch has yet
 *  (in order, each far from the one before), else the next round. */
export function freeColor(branches: BranchLook[]): string {
  const used = new Set(branches.filter((b) => b.name !== MAIN).map((b) => branchColor(branches, b.name)));
  return palette.find((c) => !used.has(c)) ?? palette[branches.length % palette.length];
}
