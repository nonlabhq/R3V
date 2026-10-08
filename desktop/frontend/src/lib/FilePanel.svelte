<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, ago, errorText, formatBytes, type FileVersion, type ProjectFile, type State } from "./api";
  import { toast } from "./notify.svelte";
  import { viewerFor, type Side } from "./viewers";
  import { branchGraph } from "./branchGraph";
  import { branchLabel, branchLane } from "./branches";
  import { initial } from "./palette";
  import { baseName, isChanged, toolOf } from "./files";

  // The Files tab's file panel: the file large, where it is, what you
  // changed (when you did), and its history; a version picked in the
  // history shows instead, and can be restored (this one file only).
  let { root, st, file, stamp, onrestore }: {
    root: string;
    st: State;
    file: ProjectFile;
    stamp: number; // changes when the folder may have changed
    onrestore: (path: string, version: string, label: string, source: string) => void;
  } = $props();

  let history = $state<FileVersion[] | null>(null);
  let picked = $state(""); // a version of the history shown instead of now
  let historyOf = ""; // path|head: read again after a commit
  $effect(() => {
    const p = file.path, key = `${p}|${st.head}`;
    if (key === historyOf) return;
    if (!historyOf.startsWith(p + "|")) { history = null; picked = ""; }
    historyOf = key;
    if (file.status === "ignored") { history = []; return; }
    api.FileHistory(root, p).then((h) => { if (historyOf === key) history = h ?? []; })
      .catch((e) => { if (historyOf === key) { history = []; toast(errorText(e), "error"); } });
  });

  const label = (h: FileVersion) => `“${h.version.message || h.version.short}”`;
  let View = $derived(viewerFor(file).component);
  let tool = $derived(toolOf(file.path));
  let pickedV = $derived(picked ? history?.find((h) => h.version.id === picked) : undefined);
  // In the version you're on (a moved file: where it was).
  let headPath = $derived(file.status === "renamed" && file.from ? file.from : file.path);
  let headSide = $derived<Side | null>(st.head ? { path: headPath, version: st.head, label: t("In the version you're on") } : null);
  let nowSide = $derived<Side>({ path: file.path, version: "", label: isChanged(file) ? t("Now (not committed)") : t("In the project") });
  // What the large preview shows: a version picked, else the file now (a
  // deleted one as it was).
  let shown = $derived<Side | null>(pickedV
    ? (pickedV.status === "deleted" ? null : { path: pickedV.path || file.path, version: pickedV.version.id, label: label(pickedV) })
    : file.status === "deleted" ? headSide : nowSide);
  let changes = $derived(!picked && isChanged(file) && file.status !== "ignored");
  let hadOne = $derived(["modified", "deleted", "renamed"].includes(file.status));

  const sym: Record<string, string> = { added: "A", modified: "M", deleted: "D", renamed: "R" };
  const statusName = (s: string) => ({ added: t("New"), modified: t("Changed"), deleted: t("Deleted"), renamed: t("Moved") } as Record<string, string>)[s] ?? "";
  const changeText = () => file.status === "renamed"
    ? t(file.edited ? "Moved from {path}, and changed, since the version you're on" : "Moved from {path} since the version you're on", { path: file.from })
    : file.status === "added" ? t("New since the version you're on") : file.status === "deleted" ? t("Deleted since the version you're on")
    : t("Changed since the version you're on");

  // The branch each version was made on (as the graph draws it), for its colour.
  let chainOf = $derived(branchGraph(st.history, st.branches.map((b) => ({ name: b.name, latest: b.latest?.id ?? "" })), st.branch,
    "main", st.head, (n) => branchLane(st.branches, n)).chainOf);
  const branchOf = (id: string) => chainOf.get(id)?.name || st.branch;

  // The line above the history is dragged to share the panel's height
  // (remembered on this computer).
  const TOP_KEY = "r3v.filePanelTop";
  let topH = $state((() => { try { return Number(localStorage.getItem(TOP_KEY)) || 320; } catch { return 320; } })());
  let bodyH = $state(0);
  let topPx = $derived(bodyH ? Math.round(Math.min(Math.max(140, topH), Math.max(140, bodyH - 120))) : topH);
  function resizeTop(e: PointerEvent) {
    if (e.button !== 0) return;
    e.preventDefault();
    const y0 = e.clientY, h0 = topPx;
    const move = (m: PointerEvent) => { if (!(m.buttons & 1)) return up(); topH = Math.round(h0 + m.clientY - y0); };
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", up);
      topH = topPx;
      try { localStorage.setItem(TOP_KEY, String(topH)); } catch { /* not remembered */ }
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", up);
  }

  function open() {
    api.OpenInLive(root, file.path).catch((e) => toast(errorText(e), "error"));
  }
  function reveal() {
    api.ShowFile(root, file.path).catch((e) => toast(errorText(e), "error"));
  }
</script>

<div class="fp">
  <header>
    <span class="badge" title={tool.name || undefined}>{tool.badge}</span>
    <strong class="name" title={file.path}>{baseName(file.path)}</strong>
    {#if sym[file.status]}<span class="st {file.status}" title={statusName(file.status)}>{sym[file.status]}</span>{/if}
  </header>

  <div class="body" bind:clientHeight={bodyH}>
    <div class="top" style:height={file.status === "ignored" ? undefined : `${topPx}px`}>
    <div class="preview">
      {#if file.status === "ignored"}
        <p class="muted">{t("R3V doesn't keep this file in versions: the project's rules leave it out (see the project's settings, ⚙ at the top).")}</p>
      {:else if shown}
        {#key `${shown.path}|${shown.version}`}
          <View {root} {file} a={shown} b={null} compare={false} {stamp} />
        {/key}
      {:else}
        <p class="muted">{t("Deleted in this version.")}</p>
      {/if}
    </div>
    <p class="meta">
      <span class="mono">{file.path}</span>{#if file.size} · {formatBytes(file.size)}{/if}{#if history?.length} · {tn(history.length, "{n} version", "{n} versions")}{/if}
      {#if picked}<button class="link" onclick={() => (picked = "")}>{t("Back to now")}</button>{/if}
    </p>

    {#if changes}
      <section class="changes">
        <h3>{t("Your changes")}</h3>
        <p class="muted small">{changeText()}{file.size ? ` · ${formatBytes(file.size)}` : ""}</p>
        <View {root} {file} a={file.status === "deleted" ? null : nowSide} b={hadOne ? headSide : null} compare={true} {stamp} />
      </section>
    {/if}
    </div>

    {#if file.status !== "ignored"}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="hsplit" onpointerdown={resizeTop}></div>
      <section class="history">
        <h3>{t("History of this file")}</h3>
        {#if history === null}
          <p class="muted small">{t("Loading…")}</p>
        {:else if history.length === 0}
          <p class="muted small">{t("No committed versions of this file yet.")}</p>
        {:else}
          <ol class="timeline">
            {#each history as h, i (h.version.id)}
              {@const b = branchOf(h.version.id)}
              <li class:on={picked === h.version.id} style:--c="var(--lane-{branchLane(st.branches, b)})">
                <button class="row" onclick={() => (picked = picked === h.version.id || i === 0 && !isChanged(file) ? "" : h.version.id)}
                  aria-pressed={picked === h.version.id}>
                  <span class="node" aria-hidden="true">{initial(h.version.author || "?")}</span>
                  <span class="txt">
                    <span class="msg">{h.version.message || t("(no description)")}</span>
                    <span class="sub">{h.version.author} · {ago(h.version.time)} · <span class="br">{branchLabel(st.branches, b)}</span></span>
                  </span>
                  {#if i === 0}<span class="cur">{t("current")}</span>{/if}
                </button>
                {#if (i > 0 || isChanged(file)) && h.status !== "deleted"}
                  <button class="restore" title={t("Put this file back as it was in this version; the rest of the project stays")}
                    onclick={() => onrestore(file.path, h.version.id, h.version.message || h.version.short, h.path || file.path)}>↶ {t("Restore")}</button>
                {/if}
              </li>
            {/each}
          </ol>
        {/if}
      </section>
    {/if}
  </div>

  <footer>
    <button class="primary" disabled={file.status === "deleted"} onclick={open}>
      {tool.name ? t("Open in {tool}", { tool: tool.name }) : t("Open")}</button>
    <button disabled={file.status === "deleted"} onclick={reveal}>{t("Show in folder")}</button>
  </footer>
</div>

<style>
  .fp { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  header { flex: none; display: flex; align-items: center; gap: var(--sp-10); padding: var(--sp-16) var(--sp-18) var(--sp-12); }
  .badge { flex: none; min-width: 22px; height: 18px; padding: 0 var(--sp-4); border-radius: var(--radius-sm); background: var(--hover-strong);
    color: var(--muted); font-size: var(--fs-2xs); font-weight: var(--fw-bold); display: inline-flex; align-items: center; justify-content: center; }
  .name { flex: 1; min-width: 0; font-size: var(--fs-lg); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .st { flex: none; font-size: var(--fs-2xs); font-weight: var(--fw-bold); padding: 1px var(--sp-6); border-radius: var(--radius-sm);
    background: var(--mod-soft); color: var(--mod); }
  .st.added { background: var(--add-soft); color: var(--add); }
  .st.deleted { background: var(--del-soft); color: var(--del); }
  .st.renamed { background: var(--warn-soft); color: var(--warn); }
  .body { flex: 1; min-height: 0; display: flex; flex-direction: column; }
  /* the file (and your changes) above, its history below: the line between them is dragged */
  .top { flex: none; min-height: 0; overflow: auto; }
  .hsplit { flex: none; height: 9px; margin: -4px 0; position: relative; z-index: 1; cursor: row-resize; touch-action: none; }
  .hsplit:hover, .hsplit:active { background: linear-gradient(to bottom, transparent 4px, var(--accent) 4px, var(--accent) 5px, transparent 5px); }
  .history { flex: 1; min-height: 0; overflow: auto; }
  .preview { margin: 0 var(--sp-18); min-height: 120px; overflow: auto; border-radius: var(--radius-lg);
    background: var(--bg-sunken); padding: var(--sp-8); }
  .preview > p { padding: var(--sp-12); }
  .meta { margin: var(--sp-10) var(--sp-18) var(--sp-14); font-size: var(--fs-sm); color: var(--muted); overflow-wrap: anywhere; }
  .meta .mono { font-size: var(--fs-xs); }
  .link { border: none; background: transparent; padding: 0 var(--sp-4); color: var(--accent-text); font-size: var(--fs-sm); }
  section { border-top: var(--border-width) solid var(--line-strong); padding: var(--sp-12) var(--sp-18) var(--sp-14); }
  h3 { margin: 0 0 var(--sp-10); font-size: var(--fs-xs); font-weight: var(--fw-semibold); color: var(--faint);
    text-transform: uppercase; letter-spacing: .08em; }
  .small { font-size: var(--fs-sm); }
  .timeline { list-style: none; margin: 0 calc(var(--sp-8) * -1); padding: 0; }
  .timeline li { position: relative; display: flex; align-items: center; border-radius: var(--radius-lg); }
  /* the line from one version to the one before, in the newer one's colour */
  .timeline li:not(:last-child)::before { content: ""; position: absolute; left: calc(var(--sp-8) + 11px); top: 34px; bottom: -10px;
    width: 2px; background: var(--c); opacity: .7; }
  .timeline li:hover, .timeline li.on { background: var(--hover); }
  .row { flex: 1; min-width: 0; display: flex; align-items: center; gap: var(--sp-10); border: none; background: transparent;
    padding: var(--sp-6) var(--sp-8); text-align: left; border-radius: var(--radius-lg); }
  .row:hover:not(:disabled) { background: transparent; }
  .node { flex: none; width: 24px; height: 24px; border-radius: 50%; border: 2px solid var(--c); background: var(--panel);
    display: inline-flex; align-items: center; justify-content: center; font-size: var(--fs-2xs); font-weight: var(--fw-bold); color: var(--c); }
  .txt { flex: 1; min-width: 0; display: flex; flex-direction: column; line-height: 1.3; }
  .msg { font-weight: var(--fw-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sub { font-size: var(--fs-xs); color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .br { color: var(--c); }
  .cur { flex: none; font-size: var(--fs-xs); color: var(--faint); margin-right: var(--sp-6); }
  .restore { flex: none; visibility: hidden; margin-right: var(--sp-8); padding: var(--sp-2) var(--sp-10); border-radius: var(--radius-pill);
    font-size: var(--fs-xs); }
  .timeline li:hover .restore, .restore:focus-visible { visibility: visible; }
  footer { flex: none; display: flex; gap: var(--sp-10); padding: var(--sp-12) var(--sp-18) var(--sp-16);
    border-top: var(--border-width) solid var(--line-strong); }
  footer .primary { flex: 1; }
</style>
