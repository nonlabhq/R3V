import { language } from "./i18n";

// A version's message is a title, its first line, and details, the rest
// (Git's way; docs/design/agent-workspaces.md, "Title and details").
/** A version's title: its message's first line. */
export const titleOf = (message: string | undefined | null) => (message ?? "").split("\n", 1)[0].trimEnd();

/** A size in words: 512 B, 1.5 MB, 20 GB. */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = n / 1024, i = 0;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v < 10 ? v.toFixed(1) : Math.round(v)} ${units[i]}`;
}

/** How long ago iso was, in the language in use ("5 min ago", "5 分鐘前"…);
 * a date and time after a day. */
export function ago(iso: string): string {
  const t = Date.parse(iso);
  if (isNaN(t)) return "";
  const s = Math.max(0, (Date.now() - t) / 1000);
  const rel = new Intl.RelativeTimeFormat(language(), { numeric: "auto", style: "short" });
  if (s < 60) return rel.format(0, "second");
  if (s < 3600) return rel.format(-Math.floor(s / 60), "minute");
  if (s < 86400) return rel.format(-Math.floor(s / 3600), "hour");
  const d = new Date(t);
  return d.toLocaleDateString(language(), { month: "short", day: "numeric" }) +
    " " + d.toLocaleTimeString(language(), { hour: "2-digit", minute: "2-digit" });
}
