// The shared viewers' own settings, remembered on this computer. They
// change how a kind of file is shown, never what is looked at.
const read = (k: string) => { try { return localStorage.getItem(k) ?? ""; } catch { return ""; } };
const write = (k: string, v: string) => { try { localStorage.setItem(k, v); } catch { /* not remembered */ } };

// Pictures compared side by side or under a slider.
export const imageLook = $state({ mode: (read("r3v.compareMode") === "slider" ? "slider" : "side") as "side" | "slider" });
export function setImageMode(m: "side" | "slider") {
  imageLook.mode = m;
  write("r3v.compareMode", m);
}

// Markdown notes: as they read, or as text.
export const mdLook = $state({ text: read("r3v.mdText") === "1" });
export function setMdText(on: boolean) {
  mdLook.text = on;
  write("r3v.mdText", on ? "1" : "0");
}
