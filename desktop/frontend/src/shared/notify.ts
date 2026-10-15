// Notices from the shared components ("Invitation sent", an error): shown
// by whoever uses them (the App's toasts, the web app's) through
// setNotifier. Until one is set, nothing shows.

export type NoticeKind = "info" | "ok" | "warn" | "error";
type Notifier = (text: string, kind: NoticeKind, ms?: number) => void;

let to: Notifier = () => {};

/** Sets where the shared components' notices go. */
export function setNotifier(fn: Notifier) {
  to = fn;
}

/** Tells the person something, briefly. */
export function notify(text: string, kind: NoticeKind = "info", ms?: number) {
  to(text, kind, ms);
}
