import type { FileSource, TextChanges, TextContent, TextLine } from "./source";

// Text read and compared in the browser, for a source without its own
// (R3V Cloud's web app): as the App does in Go (desktop/files.go,
// internal/textdiff), with the same limits.

const MAX_TEXT = 8 << 20; // bytes per side
const MAX_DIFF_LINES = 5000; // lines shown of a comparison
const MAX_WHOLE = 20000; // lines shown of a whole file
const MAX_EDITS = 4000; // beyond it, the changed middle is all removed then all added

/** The text of a file in a version (null: no text: not UTF-8, or has NUL bytes); "none": no file. */
async function readText(source: FileSource, path: string, version: string): Promise<{ text: string | null; tooBig: boolean }> {
  if (version === "none") return { text: "", tooBig: false };
  const f = await source.open(path, version);
  try {
    if (f.size > MAX_TEXT) return { text: null, tooBig: true };
    const bytes = new Uint8Array(await (await fetch(f.url)).arrayBuffer());
    if (bytes.length > MAX_TEXT) return { text: null, tooBig: true };
    if (bytes.includes(0)) return { text: null, tooBig: false };
    try {
      return { text: new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(bytes), tooBig: false };
    } catch {
      return { text: null, tooBig: false };
    }
  } finally {
    f.close();
  }
}

/** Lines without their endings (as textdiff.Split). */
export function split(text: string): string[] {
  if (text === "") return [];
  return text.replace(/\n$/, "").split("\n").map((l) => l.replace(/\r$/, ""));
}

export async function textOf(source: FileSource, path: string, version: string): Promise<TextContent> {
  const { text, tooBig } = await readText(source, path, version);
  const out: TextContent = { text: false, tooBig, lines: [], truncated: false };
  if (text === null) return out;
  out.text = true;
  out.lines = split(text);
  if (out.lines.length > MAX_WHOLE) { out.lines = out.lines.slice(0, MAX_WHOLE); out.truncated = true; }
  return out;
}

export async function diffOf(source: FileSource, file: string, from: string, to: string, whole: boolean,
  fromFile: string): Promise<TextChanges> {
  const out: TextChanges = { text: false, tooBig: false, added: 0, removed: 0, hunks: [], truncated: false };
  const [old, cur] = await Promise.all([readText(source, fromFile || file, from), readText(source, file, to)]);
  if (old.tooBig || cur.tooBig) { out.tooBig = true; return out; }
  if (old.text === null || cur.text === null) return out;
  out.text = true;
  const a = split(old.text), b = split(cur.text);
  const [context, limit] = whole ? [a.length + b.length, MAX_WHOLE] : [3, MAX_DIFF_LINES];
  let shown = 0;
  for (const h of hunks(a, b, context)) {
    for (const l of h.lines) { if (l.kind === "add") out.added++; else if (l.kind === "del") out.removed++; }
    if (shown + h.lines.length > limit && shown > 0) { out.truncated = true; continue; }
    if (h.lines.length > limit) { h.lines = h.lines.slice(0, limit); out.truncated = true; }
    shown += h.lines.length;
    out.hunks.push(h);
  }
  return out;
}

type Op = { kind: "=" | "-" | "+"; a: number; b: number };

/** The steps that turn a into b (Myers' diff), as textdiff.Diff. */
export function diff(a: string[], b: string[]): Op[] {
  const ids = new Map<string, number>();
  const num = (xs: string[]) => xs.map((x) => { let id = ids.get(x); if (id === undefined) ids.set(x, (id = ids.size)); return id; });
  const na = num(a), nb = num(b);
  let pre = 0;
  while (pre < na.length && pre < nb.length && na[pre] === nb[pre]) pre++;
  let suf = 0;
  while (suf < na.length - pre && suf < nb.length - pre && na[na.length - 1 - suf] === nb[nb.length - 1 - suf]) suf++;
  const ops: Op[] = [];
  for (let i = 0; i < pre; i++) ops.push({ kind: "=", a: i, b: i });
  const ma = na.slice(pre, na.length - suf), mb = nb.slice(pre, nb.length - suf);
  const kinds = myers(ma, mb);
  if (kinds) {
    let i = pre, j = pre;
    for (const k of kinds) {
      if (k === "=") ops.push({ kind: "=", a: i++, b: j++ });
      else if (k === "-") ops.push({ kind: "-", a: i++, b: -1 });
      else ops.push({ kind: "+", a: -1, b: j++ });
    }
  } else {
    ma.forEach((_, i) => ops.push({ kind: "-", a: pre + i, b: -1 }));
    mb.forEach((_, j) => ops.push({ kind: "+", a: -1, b: pre + j }));
  }
  for (let i = 0; i < suf; i++) ops.push({ kind: "=", a: a.length - suf + i, b: b.length - suf + i });
  return ops;
}

function myers(a: number[], b: number[]): string[] | null {
  const n = a.length, m = b.length;
  const limit = Math.min(n + m, MAX_EDITS), off = limit + 1;
  const v = new Int32Array(2 * limit + 3);
  const trace: Int32Array[] = [];
  for (let d = 0; d <= limit; d++) {
    trace.push(v.slice(off - d - 1, off + d + 2));
    for (let k = -d; k <= d; k += 2) {
      let x = k === -d || (k !== d && v[off + k - 1] < v[off + k + 1]) ? v[off + k + 1] : v[off + k - 1] + 1;
      let y = x - k;
      while (x < n && y < m && a[x] === b[y]) { x++; y++; }
      v[off + k] = x;
      if (x >= n && y >= m) return backtrack(trace, n, m);
    }
  }
  return null;
}

function backtrack(trace: Int32Array[], n: number, m: number): string[] {
  const out: string[] = [];
  let x = n, y = m;
  for (let d = trace.length - 1; d >= 0; d--) {
    const v = trace[d];
    const at = (k: number) => v[k + d + 1];
    const k = x - y;
    const pk = k === -d || (k !== d && at(k - 1) < at(k + 1)) ? k + 1 : k - 1;
    const px = at(pk), py = px - pk;
    while (x > px && y > py) { out.push("="); x--; y--; }
    if (d > 0) { if (x === px) { out.push("+"); y--; } else { out.push("-"); x--; } }
    x = px; y = py;
  }
  return out.reverse();
}

/** The changes from a to b in hunks, context lines around each (as textdiff.Hunks). */
export function hunks(a: string[], b: string[], context: number): { lines: TextLine[] }[] {
  const ops = diff(a, b);
  const out: { lines: TextLine[] }[] = [];
  for (let i = 0; i < ops.length;) {
    if (ops[i].kind === "=") { i++; continue; }
    const start = Math.max(i - context, 0);
    let end = i;
    while (end < ops.length) {
      if (ops[end].kind !== "=") { end++; continue; }
      let run = end;
      while (run < ops.length && ops[run].kind === "=") run++;
      if (run === ops.length || run - end > 2 * context) { end = Math.min(end + context, ops.length); break; }
      end = run;
    }
    out.push({ lines: ops.slice(start, end).map((op) =>
      op.kind === "=" ? { kind: "same", old: op.a + 1, new: op.b + 1, text: b[op.b] }
        : op.kind === "-" ? { kind: "del", old: op.a + 1, new: 0, text: a[op.a] }
          : { kind: "add", old: 0, new: op.b + 1, text: b[op.b] }) });
    i = end;
  }
  return out;
}
