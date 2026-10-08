// Files dropped from Explorer onto the page. Wails sends the drop to Go,
// which hands the files' paths back to the page (handlePlatformFileDrop);
// Wails's own way then looks for the drop target again from the drop's
// position, which can miss (and the drop is lost). So the page keeps
// where the drop happened (the target's data-root and data-dir) and tells
// its listeners. It also clears the targets' drop marks when a drag leaves
// the window or ends elsewhere, which Wails leaves on.

export type Drop = { files: string[]; root: string; dir: string };
type Wails = { handlePlatformFileDrop?: (files: string[], x: number, y: number) => void };

const ACTIVE = "file-drop-target-active";
const listeners = new Set<(d: Drop) => void>();
let last: { root: string; dir: string; at: number } | null = null;
let installed = false;

// clearDropMarks takes the drop marks off every target.
export function clearDropMarks() {
  if (typeof document === "undefined") return;
  for (const el of document.querySelectorAll(`.${ACTIVE}`)) el.classList.remove(ACTIVE);
}

// landed records where a drop happened (the target it was over, if any).
export function landed(target: EventTarget | null) {
  const el = (target as Element | null)?.closest?.("[data-file-drop-target]");
  last = el ? { root: el.getAttribute("data-root") ?? "", dir: el.getAttribute("data-dir") ?? "", at: Date.now() } : null;
}

// delivered passes files Go handed back to the listeners, with where they
// landed; false when the page doesn't know (Wails's way then).
export function delivered(files: string[]): boolean {
  const at = last && Date.now() - last.at < 15000 ? last : null;
  last = null;
  if (!at?.root || !files?.length) return false;
  clearDropMarks();
  for (const l of listeners) l({ files, root: at.root, dir: at.dir });
  return true;
}

function install() {
  if (installed || typeof window === "undefined") return;
  installed = true;
  window.addEventListener("drop", (e) => landed(e.target), true);
  const outside = (e: DragEvent) => e.clientX <= 0 || e.clientY <= 0 || e.clientX >= window.innerWidth || e.clientY >= window.innerHeight;
  window.addEventListener("dragleave", (e) => { if (!e.relatedTarget && outside(e)) clearDropMarks(); }, true);
  window.addEventListener("dragend", clearDropMarks, true);
  const w = (window as unknown as { _wails?: Wails })._wails;
  if (!w) return; // (not in the app)
  const theirs = w.handlePlatformFileDrop;
  w.handlePlatformFileDrop = (files, x, y) => { if (!delivered(files)) theirs?.(files, x, y); };
}

// onDroppedFiles calls fn with each drop of files; it returns how to stop.
export function onDroppedFiles(fn: (d: Drop) => void): () => void {
  install();
  listeners.add(fn);
  return () => listeners.delete(fn);
}
