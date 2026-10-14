// Where the viewers get a file's bytes: a source per project, given to
// each viewer. The App's reads its local file server; R3V Cloud's web app
// reads the team's storage in the browser (blob: URLs). A viewer only ever
// fetches the URL or puts it in an <img>, <video> or a 3D loader.

/** One state of a file: its path then, in which version ("" is the App's
 * project folder now), and what to call it ("Now", "Before"…). */
export type Side = { path: string; version: string; label: string };

/** A file opened for a viewer; close() when done with it (a blob: URL
 * goes). size and type may be unknown (0, ""). */
export type OpenedFile = { url: string; size: number; type: string; close(): void };

/** A text file's lines, as the App's TextFile gives them. */
export type TextContent = { text: boolean; tooBig: boolean; lines: string[]; truncated: boolean };
/** A line of a comparison: same, add or del; old and new count from 1 (0: not on that side). */
export type TextLine = { kind: string; old: number; new: number; text: string };
/** Two states of a text file compared, as the App's TextDiff gives them. */
export type TextChanges = { text: boolean; tooBig: boolean; added: number; removed: number;
  hunks: { lines: TextLine[] }[]; truncated: boolean };

export type FileSource = {
  /** Opens a file in a version; rejects with TooBig when it is more than the source shows. */
  open(path: string, version: string): Promise<OpenedFile>;
  /** A design file (PSD…) as a picture (the App's; else the file itself). */
  picture?(path: string, version: string): Promise<OpenedFile>;
  /** A file's URL at once (the App's): a 3D model's textures, a note's pictures. */
  urlOf?(path: string, version: string): string;
  /** A text file's lines (the App's, read in Go); else read from open(). */
  text?(path: string, version: string): Promise<TextContent>;
  /** Two states compared (the App's); else compared here. "none": no file
   * in that state. fromFile: the file's path in from, when it was elsewhere. */
  diff?(file: string, from: string, to: string, whole: boolean, fromFile: string): Promise<TextChanges>;
};

/** A file more than a source shows here. */
export class TooBig extends Error {
  constructor(public size: number, public cap: number) {
    super("too big");
  }
}

/** The folder of a path ("" at the top), with its slash. */
export const dirOf = (path: string) => (path.includes("/") ? path.slice(0, path.lastIndexOf("/") + 1) : "");

/** What each shared viewer is given. source: the project's; file: the one
 * looked at; a: the state shown (null: the file is gone in it); b: the
 * state it is compared with (null: none); compare: show the differences
 * from b. stamp changes when the App's folder may have changed. onlink: a
 * web link in a note was clicked (else it opens in a new tab); onopen: open
 * the file in its program (the App's). */
export type ViewerProps = {
  source: FileSource;
  file: { path: string; size?: number };
  a: Side | null;
  b: Side | null;
  compare: boolean;
  stamp?: number;
  onlink?: (href: string) => void;
  onopen?: () => void;
};
