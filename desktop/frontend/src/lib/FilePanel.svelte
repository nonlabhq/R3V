<script lang="ts">
  import ChangeMark from "$shared/ChangeMark.svelte";
  import { t, tn } from "./i18n.svelte";
  import { api, ago, errorText, formatBytes, titleOf, type FileVersion, type MemberLook, type ProjectFile, type State } from "./api";
  import { toast } from "./notify.svelte";
  import { viewerFor, type Side } from "./viewers";
  import { branchGraph } from "$shared/branchGraph";
  import { branchLabel, branchLane } from "$shared/branches";
  import { initial } from "$shared/palette";
  import { baseName, dirOf, isChanged } from "$shared/files";
  import FileIcon from "$shared/FileIcon.svelte";
  import LockBadge from "./LockBadge.svelte";
  import { lockText, type LockItem } from "./locks.svelte";

  // The Files tab's file panel: the file large, where it is, what you
  // changed (when you did), and its history; a version picked in the
  // history shows instead, and can be restored (this one file only).
  let { root, st, file, stamp, onrestore, lock, looks, branchName, onbreak }: {
    root: string;
    st: State;
    file: ProjectFile;
    stamp: number; // changes when the folder may have changed
    onrestore: (path: string, version: string, label: string, source: string) => void;
    lock?: LockItem;  // who holds the file (a team's or an agent lock)
    looks?: Record<string, MemberLook | undefined>;
    branchName?: (key: string) => string;
    onbreak?: (lock: LockItem) => void; // breaking it (asked first; offered for agent locks)
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

  const label = (h: FileVersion) => `“${titleOf(h.version.message) || h.version.short}”`;
  let View = $derived(viewerFor(file).component);
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
  // Beside the name while a version is picked: which one the file shows
  // (the newest is the version you're on, picked over your changes).
  let asOf = $derived(!pickedV || !history ? "" : history[0] === pickedV
    ? t("As in the version you're on · {ago}", { ago: ago(pickedV.version.time) })
    : t("An older version · {ago}", { ago: ago(pickedV.version.time) }));
  let hadOne = $derived(["modified", "deleted", "renamed"].includes(file.status));

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
  // The history folded or not: one choice for the project's files (kept).
  const histKey = $derived(`r3v.fileHistory:${root}`);
  let histOpen = $state(true);
  $effect(() => { try { histOpen = localStorage.getItem(histKey) !== "0"; } catch { histOpen = true; } });
  function toggleHistory() {
    histOpen = !histOpen;
    try { localStorage.setItem(histKey, histOpen ? "1" : "0"); } catch { /* not kept */ }
  }
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
  <div class="band caption file-band">{t("File")}<span class="end where" title={file.path}>{dirOf(file.path)}</span></div>
  <header>
    <span class="badge"><FileIcon path={file.path} kind={file.kind} /></span>
    <strong class="name" title={file.path}>{baseName(file.path)}</strong>
    <span class="asof" title={pickedV ? label(pickedV) : undefined}>{asOf}</span>
    <ChangeMark status={file.status} />
  </header>
  {#if lock && !lock.person}
    <p class="lockline">
      <LockBadge {lock} {looks} {branchName} />
      <span class="locktext">{lockText(lock, branchName)}</span>
      {#if lock.agent && onbreak}
        <button class="link" onclick={() => onbreak(lock)}>{lock.mine ? t("Unlock") : t("Break lock…")}</button>
      {/if}
    </p>
  {/if}

  <div class="body" bind:clientHeight={bodyH}>
    <div class="top" class:grow={!histOpen} style:height={file.status === "ignored" || !histOpen ? undefined : `${topPx}px`}>
    <div class="preview">
      {#if file.status === "ignored"}
        <p class="muted">{t("R3V doesn't keep this file in versions: the project's rules leave it out (see the project's settings, ⚙ at the top).")}</p>
      {:else if shown}
        {#key `${shown.path}|${shown.version}`}
          <View {root} {file} a={shown} b={null} compare={false} {stamp} editable={!picked && file.status !== "deleted"} />
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
      {#if histOpen}<div class="hsplit" onpointerdown={resizeTop}></div>{/if}
      <section class="history" class:folded={!histOpen}>
        <h3 class="band"><button class="fold" aria-expanded={histOpen} onclick={toggleHistory}>
          <span class="chev" aria-hidden="true">{histOpen ? "▾" : "▸"}</span>{t("History of this file")}{#if !histOpen && history?.length}<span class="n">{history.length}</span>{/if}
        </button>{#if histOpen && history?.length}<span class="end">{tn(history.length, "{n} version", "{n} versions")}</span>{/if}</h3>
        {#if !histOpen}
          <!-- (folded: the file takes the room) -->
        {:else if history === null}
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
                    <span class="msg">{titleOf(h.version.message) || t("(no description)")}</span>
                    <span class="sub">{h.version.author} · {ago(h.version.time)} · <span class="br">{branchLabel(st.branches, b)}</span></span>
                  </span>
                  {#if i === 0}<span class="cur">{t("current")}</span>{/if}
                </button>
                {#if (i > 0 || isChanged(file)) && h.status !== "deleted"}
                  <button class="restore" title={t("Put this file back as it was in this version; the rest of the project stays")}
                    onclick={() => onrestore(file.path, h.version.id, titleOf(h.version.message) || h.version.short, h.path || file.path)}>↶ {t("Restore")}</button>
                {/if}
              </li>
            {/each}
          </ol>
        {/if}
      </section>
    {/if}
  </div>

  <footer>
    <!-- (with the program Windows opens it with) -->
    <button class="primary" disabled={file.status === "deleted"} onclick={open}>{t("Open")}</button>
    <button class="icon-btn" disabled={file.status === "deleted"} onclick={reveal} title={t("Show in folder")} aria-label={t("Show in folder")}>
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M1.75 4.25v8a1 1 0 0 0 1 1h10.5a1 1 0 0 0 1-1v-6.5a1 1 0 0 0-1-1H8L6.5 3.25H2.75a1 1 0 0 0-1 1z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" /></svg>
    </button>
  </footer>
</div>

<style>
  .fp { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  /* (Flat: the band across the panel; this look: a caption over the name) */
  .file-band { margin: calc(var(--sp-16) * (1 - var(--bleed))) calc(var(--sp-18) * (1 - var(--bleed))) calc(var(--sp-8) * (var(--bleed) - 1)); }
  .where { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; direction: rtl; }
  header { flex: none; display: flex; align-items: center; gap: var(--sp-10); padding: var(--sp-16) var(--sp-18) var(--sp-12); }
  .badge { flex: none; min-width: 22px; height: 18px; padding: 0 var(--sp-4); border-radius: var(--radius-sm); background: var(--hover-strong);
    color: var(--muted); font-size: var(--fs-2xs); font-weight: var(--fw-bold); display: inline-flex; align-items: center; justify-content: center; }
  .name { flex: 0 1 auto; min-width: 0; font-size: var(--fs-lg); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  /* (it takes the room left, so the change mark stays at the right) */
  .asof { flex: 1 1 0; min-width: 0; font-size: var(--fs-sm); color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .lockline { flex: none; display: flex; align-items: center; gap: var(--sp-8); margin: 0 var(--sp-18) var(--sp-10);
    font-size: var(--fs-sm); color: var(--muted); }
  .locktext { flex: 1; min-width: 0; }
  .body { flex: 1; min-height: 0; display: flex; flex-direction: column; }
  /* the file (and your changes) above, its history below: the line between them is dragged */
  .top { flex: none; min-height: 0; overflow: auto; }
  .hsplit { flex: none; height: 9px; margin: -4px 0; position: relative; z-index: 1; cursor: row-resize; touch-action: none; }
  .hsplit:hover, .hsplit:active { background: linear-gradient(to bottom, transparent 4px, var(--accent) 4px, var(--accent) 5px, transparent 5px); }
  .history { flex: 1; min-height: 0; overflow: auto; }
  .history.folded { flex: none; padding-bottom: var(--sp-10); }
  .history.folded h3 { margin: 0 calc(var(--sp-18) * -1 * var(--bleed)); }
  .history h3 { display: flex; align-items: center; margin: calc(var(--sp-12) * -1 * var(--bleed)) calc(var(--sp-18) * -1 * var(--bleed)) var(--sp-10); }
  .history h3 .end { margin-left: auto; font-weight: normal; letter-spacing: 0; text-transform: none; }
  .history.folded { padding-top: calc(var(--sp-12) * (1 - var(--bleed))); }
  .top.grow { flex: 1; }
  .fold { display: inline-flex; align-items: center; gap: var(--sp-6); padding: 0; border: none; background: none; color: inherit;
    font: inherit; text-transform: inherit; letter-spacing: inherit; cursor: pointer; }
  .fold:hover:not(:disabled) { background: none; color: var(--text); }
  .fold .chev { font-size: var(--fs-2xs); }
  .fold .n { margin-left: var(--sp-4); color: var(--faint); font-variant-numeric: tabular-nums; }
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
  .timeline li { position: relative; display: flex; align-items: center; border-radius: var(--row-radius); }
  /* the line from one version to the one before, in the newer one's colour */
  .timeline li:not(:last-child)::before { content: ""; position: absolute; left: calc(var(--sp-8) + 11px); top: 34px; bottom: -10px;
    width: 2px; background: var(--c); opacity: .7; }
  .timeline li:hover { background: var(--hover); }
  /* the version shown, picked in the accent like a project picked in the side bar */
  .timeline li.on { background: var(--select-bg); box-shadow: var(--select-bar), inset 0 0 0 var(--border-width) var(--select-line); }
  .timeline li.on .msg { color: var(--text); }
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
  .timeline li:hover .restore, .timeline li.on .restore, .restore:focus-visible { visibility: visible; }
  footer { flex: none; display: flex; gap: var(--sp-10); padding: var(--sp-12) var(--sp-18) var(--sp-16);
    border-top: var(--border-width) solid var(--line-strong); }
  footer .primary { flex: 1; }
  .icon-btn { flex: none; display: inline-flex; align-items: center; justify-content: center; width: 38px; padding: 0; }
  .icon-btn svg { width: 16px; height: 16px; }
</style>
