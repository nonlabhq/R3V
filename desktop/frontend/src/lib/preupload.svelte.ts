import { Events } from "@wailsio/runtime";
import { api, type Progress } from "./api";

// What is going up, by project root: big files going up in the background
// (see desktop/preupload.go), with the ones waiting after them, and the
// steps of a commit or update (the "progress" events); each with its speed.
// The upload queue window (UploadQueue.svelte) shows them all.
export type Preupload = {
  root: string; path: string; bytes: number; total: number; done: boolean;
  waiting: { path: string; size: number }[];
  speed: number; // bytes a second (0 until known)
};
export type Transfer = Progress & { speed: number };

export const preuploads = $state<Record<string, Preupload>>({});
export const transfers = $state<Record<string, Transfer>>({});
export const queue = $state({ open: false });
// Commits being cancelled, by root (until their step ends).
export const cancelling = $state<Record<string, boolean>>({});

// cancelSave stops root's commit or share before the team gets it.
export async function cancelSave(root: string) {
  cancelling[root] = true;
  try {
    if (!(await api.CancelSave(root))) delete cancelling[root]; // already done
  } catch {
    delete cancelling[root];
  }
}

// Speed: bytes a second, smoothed, from the bytes so far at each event.
const seen: Record<string, { at: number; bytes: number; speed: number }> = {};
function speedOf(key: string, bytes: number): number {
  const now = performance.now();
  const last = seen[key];
  if (!last || bytes < last.bytes) {
    seen[key] = { at: now, bytes, speed: last?.speed ?? 0 };
    return seen[key].speed;
  }
  const dt = (now - last.at) / 1000;
  if (dt < 0.3) return last.speed;
  const now_ = (bytes - last.bytes) / dt;
  const speed = last.speed ? last.speed * 0.6 + now_ * 0.4 : now_;
  seen[key] = { at: now, bytes, speed };
  return speed;
}

let watching = false;
// Ends heard before the first list came (root + path).
const ended = new Set<string>();
const stale: Record<string, ReturnType<typeof setTimeout>> = {};
const uploadStages = new Set(["scanning", "storing", "checking", "uploading"]);

// watchPreuploads follows them; once, from the app's start.
export function watchPreuploads() {
  if (watching) return;
  watching = true;
  api.Preuploads().then((list) => {
    for (const p of list ?? []) if (!ended.has(p.root + "\n" + p.path) && !preuploads[p.root]) preuploads[p.root] = { ...p, waiting: p.waiting ?? [], speed: 0 };
  }).catch(() => {});
  Events.On("preupload", (ev: { data: Omit<Preupload, "speed"> }) => {
    const p = ev.data;
    const key = `pre:${p.root}:${p.path}`;
    if (p.done) {
      ended.add(p.root + "\n" + p.path);
      delete preuploads[p.root];
      delete seen[key];
    } else {
      preuploads[p.root] = { ...p, waiting: p.waiting ?? [], speed: speedOf(key, p.bytes) };
    }
  });
  Events.On("progress", (ev: { data: Progress }) => {
    const p = ev.data;
    const key = `step:${p.root}`;
    clearTimeout(stale[p.root]);
    if (p.stage === "done") delete cancelling[p.root];
    if (p.stage === "done" || !uploadStages.has(p.stage)) {
      delete transfers[p.root];
      delete seen[key];
    } else {
      transfers[p.root] = { ...p, speed: p.bytes ? speedOf(key, p.bytes) : 0 };
      // In case its end is missed.
      stale[p.root] = setTimeout(() => delete transfers[p.root], 10 * 60000);
    }
  });
}
