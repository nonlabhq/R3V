// An emoji as a project's icon: stored as its code points in hex ("e-1f3b5"
// for 🎵), so it is a name like the others (a-z, 0-9, "-": what a team
// keeps, remote.ValidLookName). A build without emoji shows the initial.

const STORED = /^e(-[0-9a-f]{1,6})+$/;

/** The stored name of an emoji ("" when it is too long to keep). */
export function emojiName(emoji: string): string {
  const name = "e-" + [...emoji].map((c) => c.codePointAt(0)!.toString(16)).join("-");
  return name.length <= 64 ? name : "";
}

/** The emoji a stored name stands for, null when it isn't one. */
export function emojiOf(name: string | undefined): string | null {
  if (!name || !STORED.test(name)) return null;
  const points = name.slice(2).split("-").map((h) => parseInt(h, 16));
  if (points.some((p) => p > 0x10ffff)) return null;
  return String.fromCodePoint(...points);
}
