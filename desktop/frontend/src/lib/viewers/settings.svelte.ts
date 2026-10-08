// Each viewer's own settings, remembered on this computer. They change how
// a kind of file is shown, never what is looked at.
const read = (k: string) => { try { return localStorage.getItem(k) ?? ""; } catch { return ""; } };
const write = (k: string, v: string) => { try { localStorage.setItem(k, v); } catch { /* not remembered */ } };

// Pictures compared side by side or under a slider.
export const imageLook = $state({ mode: (read("r3v.compareMode") === "slider" ? "slider" : "side") as "side" | "slider" });
export function setImageMode(m: "side" | "slider") {
  imageLook.mode = m;
  write("r3v.compareMode", m);
}

// Live Sets: Live's Arrangement or Session view, or text; and, when
// comparing, every track or only the ones that changed.
export type SetPane = "arrangement" | "session" | "text";
export const setLook = $state({
  pane: (["session", "text"].includes(read("r3v.setPane")) ? read("r3v.setPane") : "arrangement") as SetPane,
  allTracks: read("r3v.setAllTracks") === "1",
});
export function setSetPane(p: SetPane) {
  setLook.pane = p;
  write("r3v.setPane", p);
}
export function setAllTracks(on: boolean) {
  setLook.allTracks = on;
  write("r3v.setAllTracks", on ? "1" : "0");
}

// Markdown notes: as they read, or as text.
export const mdLook = $state({ text: read("r3v.mdText") === "1" });
export function setMdText(on: boolean) {
  mdLook.text = on;
  write("r3v.mdText", on ? "1" : "0");
}

// The project's rules file: the rules in plain words, or its text.
export type RulesPane = "rules" | "text";
export const rulesLook = $state({ pane: (read("r3v.rulesPane") === "text" ? "text" : "rules") as RulesPane });
export function setRulesPane(p: RulesPane) {
  rulesLook.pane = p;
  write("r3v.rulesPane", p);
}
