import { untrack } from "svelte";
import { TooBig, type FileSource, type OpenedFile, type Side } from "./source";

/** A side opened for showing, closed when it changes or goes. error:
 * "tooBig", or why it couldn't be read. */
export type Opened = { file: OpenedFile | null; error: string };

/** Opens what side() names through source() (picture: as a picture, when
 * the source has one), again when the source or the path and version
 * change (not for the same side given anew); closes what it opened. */
export function opened(source: () => FileSource, side: () => Side | null, picture = false): Opened {
  const st = $state<Opened>({ file: null, error: "" });
  const key = $derived.by(() => { const s = side(); return s ? `${s.version}\u0000${s.path}` : ""; });
  $effect(() => {
    const src = source(), k = key;
    const s = untrack(side);
    st.file = null;
    st.error = "";
    if (!k || !s) return;
    let gone = false, got: OpenedFile | null = null;
    const open = picture && src.picture ? src.picture.bind(src) : src.open.bind(src);
    open(s.path, s.version).then((f) => {
      if (gone) { f.close(); return; }
      got = f;
      st.file = f;
    }).catch((e) => { if (!gone) st.error = e instanceof TooBig ? "tooBig" : String(e?.message ?? e); });
    return () => { gone = true; got?.close(); };
  });
  return st;
}
