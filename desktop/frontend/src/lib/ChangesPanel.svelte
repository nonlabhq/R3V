<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { folderMoves } from "./moves";
  import type { Snippet } from "svelte";
  import { api, ago, errorText, formatBytes, type FileVersion, type ProjectFile, type State } from "./api";
  import type { IgnoreOption } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";
  import { toast } from "./notify.svelte";
  import FileIcon from "./FileIcon.svelte";
  import Splitter from "./Splitter.svelte";
  import { splitPx } from "./splits.svelte";
  import ConvertDialog from "./ConvertDialog.svelte";
  import { fileView, setFileMode, type FileMode } from "./viewmode.svelte";
  import { viewerFor, type Side } from "./viewers";
  import { navKey, ownKey, type NavRow } from "./keynav";
  import { changesView, pickChangesView } from "./changesview.svelte";
  import { portal } from "./portal";
  import LockBadge from "./LockBadge.svelte";
  import { lockAt, locksOf } from "./locks.svelte";

  // Changes tab: files on the left; on the right the selected file, as it is
  // (Preview), against the version you're on (Changes) or through its
  // versions (History), shown by its kind's viewer (viewers/). "All files"
  // lists the whole project folder (the Files tab is FileExplorer). Each file
  // has a menu (⋯ or right click).
  let { root, st, summary, commitBox, scope, excluded = $bindable({}), ondiscard, ondiscardall, ondiscardsome, onrestore, onrules }: {
    root: string;
    scope?: "changes" | "all"; // the list fixed to the changes or every file (no All files switch)
    st: State;
    excluded?: Record<string, boolean>; // changes unticked: left out of the next commit
    summary: Snippet; // shown when no file is selected (tracks you changed)
    commitBox?: Snippet; // under the files: the message and the Commit button
    ondiscard: (path: string) => void;
    ondiscardall: () => void;
    ondiscardsome: (paths: string[]) => void; // the ticked changes (not all of them)
    onrestore: (path: string, version: string, label: string, source: string) => void; // one file from a version (source: its path then)
    onrules?: () => void; // .r3v.yaml changed (a file or folder left out)
  } = $props();

  // "All files" is remembered per project.
  let all = $state(false);
  $effect.pre(() => {
    const key = `r3v.allFiles:${root}`;
    if (scope) { all = scope === "all"; return; }
    try { all = localStorage.getItem(key) === "1"; } catch { all = false; }
  });
  function rememberAll() {
    try { localStorage.setItem(`r3v.allFiles:${root}`, all ? "1" : "0"); } catch { /* not remembered */ }
  }
  let files = $state<ProjectFile[]>([]);
  let selected = $state("");
  let lv = $derived(locksOf(root)); // file locks (off: none shown)
  // The menu of a file or folder (right click, or ⋯), with the ways to
  // leave it out of versions.
  let menu = $state<{ path: string; x: number; y: number; dir: boolean; ignore: IgnoreOption[] } | null>(null);
  let ignoreOpen = $state(false); // the Ignore submenu
  let menuW = $state(220), menuH = $state(200); // (kept inside the window)

  // history of the selected file
  let history = $state<FileVersion[] | null>(null);
  let picked = $state(""); // version id in the history

  let loadedAt = $state(0); // makes "now" previews fetch the file again
  let converting = $state(""); // sample in the Convert dialog
  function loadFiles(showAll: boolean) {
    return api.ProjectFiles(root, showAll).then((f) => { files = f ?? []; loadedAt = Date.now(); })
      .catch((e) => toast(errorText(e), "error"));
  }
  $effect(() => {
    st; // reload with the project's state
    loadFiles(all);
  });

  // Show a new file (e.g. a converted sample): open its folders, select it.
  async function reveal(p: string) {
    await loadFiles(all);
    const parts = p.split("/");
    for (let k = 1; k < parts.length; k++) open[parts.slice(0, k).join("/")] = true;
    select(p);
  }

  let current = $derived(files.find((f) => f.path === selected));
  // Where the selected file is in the version you're on (moved: elsewhere).
  let before_ = $derived(current?.status === "renamed" && current.from ? current.from : selected);
  // Live versions: "Ableton Live 12.3.1" -> "12.3". A set saved with another
  // Live than most sets in the project stands out.
  const liveShort = (c: string) => c.match(/(\d+\.\d+)/)?.[1] ?? "";
  let usualLive = $derived.by(() => {
    const count = new Map<string, number>();
    for (const f of files) if (f.live) count.set(liveShort(f.live), (count.get(liveShort(f.live)) ?? 0) + 1);
    return [...count.entries()].sort((a, b) => b[1] - a[1])[0]?.[0] ?? "";
  });
  let changedCount = $derived(files.filter((f) => f.status !== "unchanged" && f.status !== "ignored").length);

  // Folders as groups, files under them (flat list of rows).
  // The folder tree: at each level folders first, then files. Folders start
  // closed; a folder shows how many changed files it holds.
  type Folder = { path: string; name: string; folders: Map<string, Folder>; files: ProjectFile[];
    changed: number; changedSize: number; tracked: boolean };
  // Folders start open in the list of changes (a tree of them), closed in
  // All files; either way they remember being opened or closed.
  let open = $state<Record<string, boolean>>({});
  const isOpen = (p: string) => open[p] ?? !all;
  const toggleFolder = (p: string) => (open[p] = !isOpen(p));

  let tree = $derived.by(() => {
    const mk = (path: string, name: string): Folder =>
      ({ path, name, folders: new Map(), files: [], changed: 0, changedSize: 0, tracked: false });
    const top = mk("", "");
    for (const f of files) {
      const parts = f.path.split("/");
      let node = top;
      const chain = [top];
      for (let k = 0; k < parts.length - 1; k++) {
        const path = parts.slice(0, k + 1).join("/");
        if (!node.folders.has(parts[k])) node.folders.set(parts[k], mk(path, parts[k]));
        node = node.folders.get(parts[k])!;
        chain.push(node);
      }
      node.files.push(f);
      for (const n of chain) {
        if (f.status !== "unchanged" && f.status !== "ignored") { n.changed++; n.changedSize += f.size; }
        if (f.status !== "ignored") n.tracked = true;
      }
    }
    return top;
  });

  type Row = { folder?: Folder; file?: ProjectFile; depth: number };
  // The changes as a list (each with its folder under its name) or a tree;
  // every file is a tree.
  let view = $derived(all ? "tree" : changesView(root, st.changes.length));
  let rows = $derived.by((): Row[] => {
    const out: Row[] = [];
    if (view === "list") return [...files].sort((a, b) => a.path.localeCompare(b.path)).map((f) => ({ file: f, depth: 0 }));
    const walk = (node: Folder, depth: number) => {
      for (const sub of [...node.folders.values()].sort((a, b) => a.name.localeCompare(b.name))) {
        out.push({ folder: sub, depth });
        if (isOpen(sub.path)) walk(sub, depth + 1);
      }
      for (const f of [...node.files].sort((a, b) => a.path.localeCompare(b.path))) out.push({ file: f, depth });
    };
    walk(tree, 0);
    return out;
  });

  // Only the rows in view are drawn (a folder can hold thousands of files);
  // rows have a fixed height.
  let ROW = $derived(view === "list" ? 44 : 30);
  let scroller = $state<HTMLElement>();
  let list = $state<HTMLElement>();
  let scrollTop = $state(0);
  let viewH = $state(800);
  let listOffset = $state(0); // where the list starts in the scrolled panel
  function onScroll() {
    if (!scroller || !list) return;
    scrollTop = scroller.scrollTop;
    listOffset = list.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop;
  }
  $effect(() => {
    rows; // the list may have moved (e.g. the empty note went away)
    onScroll();
  });
  let win = $derived.by(() => {
    const top = scrollTop - listOffset;
    const from = Math.max(0, Math.floor(top / ROW) - 15);
    return { from, to: Math.min(rows.length, Math.ceil((top + viewH) / ROW) + 15) };
  });

  function select(p: string) {
    selected = p;
    cursor = p;
  }

  // The keyboard (see keynav.ts): the row it is on (a file, or "dir:" and
  // a folder), ↑ ↓ through the rows, → ← open and close folders, Enter too.
  // (Space is the audio player's: it plays or pauses the file shown.)
  let cursor = $state("");
  let navRows = $derived(rows.map((r): NavRow => r.folder
    ? { key: "dir:" + r.folder.path, dir: true, open: isOpen(r.folder.path), depth: r.depth }
    : { key: r.file!.path, depth: r.depth }));
  function onListKey(e: KeyboardEvent) {
    if (!ownKey(e) || (e.target as HTMLElement).closest(".head")) return;
    const at = cursor || selected;
    if (e.key === "Enter") {
      if (!at.startsWith("dir:") || (e.target as HTMLElement).matches("input")) return;
      e.preventDefault();
      toggleFolder(at.slice(4));
      return;
    }
    const nav = navKey(navRows, at, e.key, Math.max(1, Math.floor(viewH / ROW) - 1));
    if (!nav) return;
    e.preventDefault();
    if ("toggle" in nav) toggleFolder(nav.toggle.slice(4));
    else if (nav.to.startsWith("dir:")) cursor = nav.to;
    else select(nav.to);
    keepInView(navRows.findIndex((r) => r.key === ("to" in nav ? nav.to : nav.toggle)));
    scroller?.focus({ preventScroll: true }); // (the row's button may scroll out of the drawn ones)
  }
  function keepInView(i: number) {
    if (!scroller || i < 0) return;
    const top = listOffset + i * ROW, head = 40;
    if (top - head < scroller.scrollTop) scroller.scrollTop = top - head;
    else if (top + ROW > scroller.scrollTop + viewH) scroller.scrollTop = top + ROW - viewH;
  }

  // The list and the file side by side, the line between them dragged to
  // share the width; Files keeps its own split.
  let panelWidth = $state(0);
  let splitKey = $derived(scope === "all" ? "files" : "list");
  let splitDef = $derived(scope === "all" ? 0.4 : 0.34);
  let sideWidth = $derived(splitPx(splitKey, splitDef, panelWidth, 240, 300));

  // History: the selected file's versions, read when it is shown.
  let historyOf = "";
  $effect(() => {
    if (fileView.mode === "history" && selected && selected !== historyOf) loadHistory(selected);
    if (fileView.mode !== "history") historyOf = "";
  });
  async function loadHistory(p: string) {
    historyOf = p;
    history = null;
    picked = "";
    try {
      const h = await api.FileHistory(root, p);
      if (historyOf !== p) return;
      history = h;
      if (h?.length) picked = h[0].version.id;
    } catch (e) {
      toast(errorText(e), "error");
      history = [];
    }
  }
  // From the file's menu.
  function showHistory(p: string) {
    menu = null;
    selected = p;
    setFileMode("history");
  }

  // The version before `id` in this file's history (to compare with).
  const before = (id: string) => {
    const i = history?.findIndex((h) => h.version.id === id) ?? -1;
    return i >= 0 ? history![i + 1] : undefined;
  };

  const pick = (id: string) => (picked = id);

  function openMenu(e: MouseEvent, p: string, dir = false) {
    e.preventDefault();
    ignoreOpen = false;
    const m = { path: p, x: e.clientX, y: e.clientY, dir, ignore: [] as IgnoreOption[] };
    menu = m;
    api.IgnoreOptions(p, dir).then((o) => { if (menu?.path === p) menu = { ...menu, ignore: o ?? [] }; }).catch(() => {});
  }

  async function ignore(pattern: string) {
    menu = null;
    try {
      await api.AddIgnoreRule(root, pattern);
      toast(t("Left out of versions: {pattern} — a rule in .r3v.yaml; commit it to share it with the team", { pattern }), "ok", 7000);
      await loadFiles(all);
      onrules?.();
    } catch (e) {
      toast(errorText(e), "error", 9000);
    }
  }

  function openFile(p: string) {
    menu = null;
    api.OpenInLive(root, p).catch((e) => toast(errorText(e), "error"));
  }

  // Ticking: each change, or a folder's changes at once.
  const changedPaths = $derived(st.changes.map((c) => c.path));
  const isChange = (f: ProjectFile) => f.status !== "unchanged" && f.status !== "ignored";
  const inside = (dir: string) => changedPaths.filter((p) => p.startsWith(dir + "/"));
  function tick(paths: string[], on: boolean) {
    const next = { ...excluded };
    for (const p of paths) {
      if (on) delete next[p];
      else next[p] = true;
    }
    excluded = next;
  }
  // How big the ticked changes are (what the commit takes).
  let sizes = $derived(new Map(files.map((f) => [f.path, f.size])));
  const tickedSize = (ticked: string[]) => ticked.reduce((n, p) => n + (sizes.get(p) ?? 0), 0);
  // The box over the list: every change ticked, none, or some.
  let allState = $derived.by((): "on" | "off" | "some" => {
    const out = changedPaths.filter((p) => excluded[p]).length;
    return out === 0 ? "on" : out === changedPaths.length ? "off" : "some";
  });
  // A folder's box: ticked, unticked, or some (indeterminate).
  function folderState(dir: string): "on" | "off" | "some" {
    const ps = inside(dir);
    const out = ps.filter((p) => excluded[p]).length;
    return out === 0 ? "on" : out === ps.length ? "off" : "some";
  }

  // What the selected file's viewer is given in Preview and Changes: the
  // state looked at (a), the one it is compared with (b), and whether to
  // show the differences. History gives each version and the one before.
  function sides(mode: FileMode, f: ProjectFile): { a: Side | null; b: Side | null; compare: boolean } {
    const now: Side = { path: f.path, version: "", label: f.status === "unchanged" ? t("In the project") : t("Now (not committed)") };
    const head: Side | null = st.head ? { path: before_, version: st.head, label: t("In the version you're on") } : null;
    if (mode === "preview" || f.status === "unchanged") {
      // a deleted file: as it was
      return f.status === "deleted" ? { a: head, b: null, compare: false } : { a: now, b: null, compare: false };
    }
    const hadOne = f.status === "modified" || f.status === "deleted" || f.status === "renamed";
    return { a: f.status === "deleted" ? null : now, b: hadOne ? head : null, compare: true };
  }
  const versionLabel = (v: FileVersion) => `“${v.version.message || v.version.short}”`;

  const sym: Record<string, string> = { added: "+", modified: "~", deleted: "−", untracked: "○", renamed: "M", unchanged: "", ignored: "" };
  const statusName = (s: string) => ({ added: t("New"), modified: t("Changed"), deleted: t("Deleted"), renamed: t("Moved"),
    untracked: t("No longer tracked: the rules leave it out now") } as Record<string, string>)[s];
  // Folders that moved, said once on the folder (see moves.ts).
  let moves = $derived(folderMoves(files));
  // Where a moved file was, said briefly: its old name in the same folder,
  // or its old folder.
  function fromLabel(f: { path: string; from: string }): string {
    const dir = (p: string) => p.slice(0, p.lastIndexOf("/") + 1);
    return dir(f.path) === dir(f.from) ? name(f.from) : f.from;
  }
  const name = (p: string) => p.slice(p.lastIndexOf("/") + 1);
  const canDiscard = (f: ProjectFile | undefined) => !!f && ["added", "modified", "deleted", "renamed"].includes(f.status);
</script>

<svelte:window onclick={(e) => { if (menu && !(e.target as HTMLElement).closest(".ctx")) menu = null; }}
  onkeydown={(e) => { if (e.key === "Escape") menu = null; }} />

<div class="panel" bind:clientWidth={panelWidth} style:grid-template-columns="{sideWidth}px 1fr">
  {#if panelWidth}<Splitter key={splitKey} def={splitDef} width={panelWidth} minLeft={240} minRight={300} />{/if}
  <div class="side">
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
  <aside class="files" bind:this={scroller} bind:clientHeight={viewH} onscroll={onScroll} tabindex="-1" onkeydown={onListKey}>
    <!-- the list's header, kept at the top: the box to tick all, the title, how many and how big -->
    <!-- one line when there's room, else two: [box] title · how many, how big · revert · All files -->
    <div class="head" class:fixed={!!scope}>
      <span class="chevbtn h-chev"></span>
      {#if changedPaths.length}
        <input type="checkbox" class="pick h-pick" checked={allState === "on"} indeterminate={allState === "some"}
          title={allState === "on" ? t("Deselect all changes") : t("Select all changes")}
          onchange={() => tick(changedPaths, allState !== "on")} />
      {/if}
      <span class="title h-title">{all ? t("All files") : t("Changes")}</span>
      {#if changedCount}
        {@const ticked = changedPaths.filter((p) => !excluded[p])}
        <span class="total h-total">{t("{done}/{total} selected", { done: ticked.length.toLocaleString(), total: changedPaths.length.toLocaleString() })} · {formatBytes(tickedSize(ticked))}</span>
        <button class="ghost revert h-revert" disabled={!ticked.length}
          title={!ticked.length ? t("Tick changes to discard them") : ticked.length === changedPaths.length
            ? t("Discard all changes…") : tn(ticked.length, "Discard the {n} ticked change…", "Discard the {n} ticked changes…")}
          aria-label={t("Discard the ticked changes")}
          onclick={() => (ticked.length === changedPaths.length ? ondiscardall() : ondiscardsome(ticked))}><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/></svg></button>
      {/if}
      {#if !scope}
        <label class="all h-all" title={t("List every file in the project folder")}>
          {t("All files")} <input type="checkbox" class="switch" role="switch" bind:checked={all} onchange={rememberAll} />
        </label>
      {/if}
      {#if !all && changedPaths.length}
        <div class="views h-views" role="group" aria-label={t("Show the changes as")}>
          <button class:on={view === "list"} aria-pressed={view === "list"} onclick={() => pickChangesView(root, "list")} title={t("List")} aria-label={t("List")}><svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 4h10M3 8h10M3 12h10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" fill="none" /></svg></button>
          <button class:on={view === "tree"} aria-pressed={view === "tree"} onclick={() => pickChangesView(root, "tree")} title={t("Tree")} aria-label={t("Tree")}><svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 3.5h4M6 8h7M6 12.5h7M4 3.5v9h2M4 8h2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" /></svg></button>
        </div>
      {/if}
    </div>
    {#if files.length === 0}
      <p class="muted empty">{all ? t("The project folder is empty.") : (st.tool === "Ableton Live" ? t("No uncommitted changes. Work in Live and press Ctrl+S — your changes show up here.") : t("No uncommitted changes. Work in {tool} and save — your changes show up here.", { tool: st.tool ? t(st.tool) : t("your app") }))}</p>
    {:else}
      <ul bind:this={list} style:padding-top="{win.from * ROW}px" style:padding-bottom="{(rows.length - win.to) * ROW}px">
        {#each rows.slice(win.from, win.to) as row (row.file ? row.file.path : "dir:" + row.folder!.path)}
          {#if row.folder}
            {@const d = row.folder}
            <li>
              <span class="indent" style:width="{row.depth * 14}px"></span>
              <button class="ghost chevbtn" onclick={() => toggleFolder(d.path)} aria-label={isOpen(d.path) ? t("Close folder") : t("Open folder")}>
                <svg class="chev" class:open={isOpen(d.path)} viewBox="0 0 10 10" aria-hidden="true">
                  <path d="M3 1.5 L7 5 L3 8.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              </button>
              {#if changedPaths.length}
                {#if d.changed}
                  {@const fs = folderState(d.path)}
                  <input type="checkbox" class="pick" checked={fs === "on"} indeterminate={fs === "some"}
                    title={fs === "some" ? t("Some changes in this folder are ticked") : t("Commit the changes in this folder")}
                    onchange={() => tick(inside(d.path), fs !== "on")} />
                {:else}<span class="pick"></span>{/if}
              {/if}
              <button class="file dir" class:untracked={!d.tracked} class:changed={d.changed > 0} class:on={cursor === "dir:" + d.path}
                onclick={() => { cursor = "dir:" + d.path; toggleFolder(d.path); }} oncontextmenu={(e) => openMenu(e, d.path, true)} title={d.path}>
                <FileIcon kind="folder" open={isOpen(d.path)} faint={!d.tracked} />
                <span class="fname">{d.name}</span>
                {#if d.changed}
                  {@const ff = moves.movedFrom(d.path)}
                  {#if ff}<span class="from" title={t("Moved from {path}", { path: `${ff}/` })}>← {ff}/</span>{/if}
                {/if}
                <span class="right">{#if d.changed && !isOpen(d.path)}<span class="count"
                  title={tn(d.changed, "{n} changed file inside, {size}", "{n} changed files inside, {size}", { size: formatBytes(d.changedSize) })}>{d.changed}</span>{/if}</span>
              </button>
              <button class="ghost more" title={t("More")} onclick={(e) => { e.stopPropagation(); openMenu(e, d.path, true); }}>⋯</button>
            </li>
          {:else if view === "list"}
            {@const f = row.file!}
            <li class="two">
              <span class="chevbtn"></span>
              {#if changedPaths.length}
                {#if isChange(f)}
                  <input type="checkbox" class="pick" checked={!excluded[f.path]} title={t("Commit this change")}
                    onchange={(e) => tick([f.path], (e.currentTarget as HTMLInputElement).checked)} />
                {:else}<span class="pick"></span>{/if}
              {/if}
              <button class="file {f.status}" class:on={f.path === selected}
                onclick={() => select(f.path)} oncontextmenu={(e) => openMenu(e, f.path)} title={f.path}>
                <FileIcon path={f.path} kind={f.kind} faint={f.status === "ignored" || f.status === "deleted"} />
                <span class="names">
                  <span class="fname">{name(f.path)}</span>
                  <span class="fdir">{f.status === "renamed" ? `← ${f.from}` : f.path.includes("/") ? f.path.slice(0, f.path.lastIndexOf("/")) : t("Project folder")}</span>
                </span>
                <span class="right">
                  {#if lockAt(lv, f.path)}<LockBadge lock={lockAt(lv, f.path)!} />{/if}
                  {#if f.live}
                    {@const v = liveShort(f.live)}
                    <span class="live" class:odd={usualLive && v !== usualLive} title={t("Saved with {app}", { app: f.live })}>{v}</span>
                  {/if}
                  {#if sym[f.status]}<span class="sym" title={statusName(f.status)}>{sym[f.status]}</span>{/if}
                </span>
              </button>
              <button class="ghost more" title={t("More")} onclick={(e) => { e.stopPropagation(); openMenu(e, f.path); }}>⋯</button>
            </li>
          {:else}
            {@const f = row.file!}
            <li>
              <span class="indent" style:width="{row.depth * 14}px"></span>
              <span class="chevbtn"></span>
              {#if changedPaths.length}
                {#if isChange(f)}
                  <input type="checkbox" class="pick" checked={!excluded[f.path]} title={t("Commit this change")}
                    onchange={(e) => tick([f.path], (e.currentTarget as HTMLInputElement).checked)} />
                {:else}<span class="pick"></span>{/if}
              {/if}
              <button class="file {f.status}" class:on={f.path === selected}
                onclick={() => select(f.path)} oncontextmenu={(e) => openMenu(e, f.path)} title={f.path}>
                <FileIcon path={f.path} kind={f.kind} faint={f.status === "ignored" || f.status === "deleted"} />
                <span class="fname">{name(f.path)}</span>
                {#if f.status === "renamed" && !moves.covered(f.path)}
                  <span class="from" title={t(f.edited ? "Moved from {path}, and changed" : "Moved from {path}", { path: f.from })}>← {fromLabel(f)}</span>
                {/if}
                <span class="right">
                  {#if lockAt(lv, f.path)}<LockBadge lock={lockAt(lv, f.path)!} />{/if}
                  {#if f.live}
                    {@const v = liveShort(f.live)}
                    <span class="live" class:odd={usualLive && v !== usualLive}
                      title={t("Saved with {app}", { app: f.live }) + (usualLive && v !== usualLive ? " — " + t("most sets here use Live {version}", { version: usualLive }) : "")}>{v}</span>
                  {/if}
                  {#if sym[f.status]}<span class="sym" title={statusName(f.status)}>{sym[f.status]}</span>{/if}
                </span>
              </button>
              <button class="ghost more" title={t("More")} onclick={(e) => { e.stopPropagation(); openMenu(e, f.path); }}>⋯</button>
            </li>
          {/if}
        {/each}
      </ul>
    {/if}
  </aside>
  {#if commitBox}<div class="commit">{@render commitBox()}</div>{/if}
  </div>

  <section class="detail">
    {#if !selected || !current}
      {@render summary()}
    {:else}
      {@const View = viewerFor(current).component}
      <div class="detail-h">
        <div class="title">
          <div class="dname"><FileIcon path={current.path} kind={current.kind} /> {name(selected)}</div>
          <div class="faint small mono">{selected}</div>
          {#if current.live}<div class="faint small">{t("Saved with {app}", { app: current.live })}</div>{/if}
        </div>
        {#if current.status !== "ignored"}
          <div class="modes" title={t("The file as it is, what you changed since the version you're on, or its committed versions")}>
            <button class:on={fileView.mode === "preview"} onclick={() => setFileMode("preview")}>{t("Preview")}</button>
            <button class:on={fileView.mode === "changes"} onclick={() => setFileMode("changes")}>{t("Changes")}</button>
            <button class:on={fileView.mode === "history"} onclick={() => setFileMode("history")}>{t("History")}</button>
          </div>
        {/if}
      </div>

      {#if current.status === "ignored"}
        <p class="muted">{t("R3V doesn't keep this file in versions: the project's rules leave it out (see the project's settings, ⚙ at the top).")}</p>
      {:else if fileView.mode !== "history"}
        {@const v = sides(fileView.mode, current)}
        {#if fileView.mode === "preview"}
          {#if current.status === "deleted"}
            <p class="muted">{t("Deleted since the version you're on: this is how it was.")}</p>
          {:else if current.size}
            <p class="muted">{formatBytes(current.size)}</p>
          {/if}
        {:else if current.status === "unchanged"}
          <p class="muted">{t("No changes since the version you're on.")} {formatBytes(current.size)}</p>
        {:else}
          <p class="muted">{current.status === "renamed" ? t(current.edited ? "Moved from {path}, and changed, since the version you're on" : "Moved from {path} since the version you're on", { path: current.from })
            : current.status === "added" ? t("New since the version you're on") : current.status === "deleted" ? t("Deleted since the version you're on") : t("Changed since the version you're on")}{current.size ? ` · ${formatBytes(current.size)}` : ""}.</p>
        {/if}
        {#if v.a || v.b}
          <View {root} file={current} a={v.a} b={v.b} compare={v.compare} stamp={loadedAt} />
        {/if}
      {:else}
        {#if history === null}
          <p class="muted">{t("Loading…")}</p>
        {:else if history.length === 0}
          <p class="muted">{t("No committed versions of this file yet.")}</p>
        {:else}
          <ul class="versions">
            {#each history as h (h.version.id)}
              <li>
                <button class:on={h.version.id === picked} onclick={() => pick(h.version.id)}>
                  <span class="vsym {h.status}">{sym[h.status]}</span>
                  <span class="vmsg">{h.version.message || t("(no description)")}</span>
                  {#if h.status === "renamed"}<span class="from" title={t("Moved here from {path}", { path: h.from })}>← {h.from}</span>{/if}
                  <span class="faint">{h.version.author} · {ago(h.version.time)}</span>
                </button>
              </li>
            {/each}
          </ul>
          {#if picked && history.find((x) => x.version.id === picked)}
            {@const h = history.find((x) => x.version.id === picked)!}
            {@const prev = before(picked)}
            <!-- where the file was in each version (it may have moved since) -->
            {@const hp = h.path || selected}
            {@const pp = prev?.path || selected}
            <div class="picked">
              {#if h.status !== "deleted"}
                <div class="restore">
                  <button onclick={() => onrestore(selected, h.version.id, h.version.message || h.version.short, hp)}
                    title={t("Put this file back as it was in this version; the rest of the project stays")}>{t("Restore this version")}</button>
                  <span class="faint small">{t("Only this file changes; commit it when you're happy.")}</span>
                </div>
              {/if}
              <p class="muted">{h.status === "renamed" ? t("Moved here from {path} in this version.", { path: h.from }) : h.status === "added" ? t("Added in this version.")
                : h.status === "deleted" ? t("Deleted in this version.") : t("Changed in this version.")}</p>
              <View {root} file={current} compare={true} stamp={loadedAt}
                a={h.status === "deleted" ? null : { path: hp, version: h.version.id, label: versionLabel(h) }}
                b={prev && prev.status !== "deleted" ? { path: pp, version: prev.version.id, label: t("Before: {version}", { version: versionLabel(prev) }) } : null} />
            </div>
          {/if}
        {/if}
      {/if}
    {/if}
  </section>
</div>

{#if menu}
  {@const m = menu}
  {@const f = m.dir ? undefined : files.find((x) => x.path === m.path)}
  <div class="ctx surface-menu" role="menu" use:portal bind:offsetWidth={menuW} bind:offsetHeight={menuH}
    style:left="{Math.max(8, Math.min(m.x, window.innerWidth - menuW - 8))}px" style:top="{Math.max(8, Math.min(m.y, window.innerHeight - menuH - 8))}px">
    {#if !m.dir && f && f.status !== "deleted"}
      <button class="item" onclick={() => openFile(m.path)}>{t("Open")}</button>
    {/if}
    {#if m.dir || (f && f.status !== "deleted")}
      <button class="item" onclick={() => { const p = m.path; menu = null; api.ShowFile(root, p).catch((e) => toast(errorText(e), "error")); }}>{t("Show in Explorer")}</button>
    {/if}
    {#if !m.dir}
      <button class="item" disabled={f?.status === "ignored"} onclick={() => showHistory(m.path)}>{t("View file history")}</button>
      {#if f?.kind === "audio" && f.status !== "deleted"}
        <button class="item" onclick={() => { converting = m.path; menu = null; }}>{t("Convert…")}</button>
      {/if}
      {#if canDiscard(f)}
        <button class="item danger-text" onclick={() => { const p = m.path; menu = null; ondiscard(p); }}>{t("Discard changes…")}</button>
      {/if}
    {/if}
    {#if m.ignore.length}
      <div class="sep"></div>
      <div class="sub" role="none" onmouseenter={() => (ignoreOpen = true)} onmouseleave={() => (ignoreOpen = false)}>
        <button class="item has-sub" onclick={() => (ignoreOpen = !ignoreOpen)} aria-expanded={ignoreOpen}>
          {t("Ignore")}<span class="arrow">›</span>
        </button>
        {#if ignoreOpen}
          <div class="ctx submenu surface-menu" role="menu" class:left={m.x > window.innerWidth - menuW - 270}>
            {#each m.ignore as o}
              <button class="item" onclick={() => ignore(o.pattern)}>{o.label}<span class="faint pat mono">{o.pattern}</span></button>
            {/each}
            <p class="faint note">{t("Adds a rule to .r3v.yaml: the files stay on disk, out of versions.")}</p>
          </div>
        {/if}
      </div>
    {/if}
  </div>
{/if}

{#if converting}
  <ConvertDialog {root} file={converting} onclose={() => (converting = "")}
    ondone={([p]) => { converting = ""; if (!p) return; toast(t("Converted to {file}", { file: p.slice(p.lastIndexOf("/") + 1) }), "ok"); reveal(p); }} />
{/if}

<style>
  aside.files:focus { outline: none; }
  .panel { position: relative; display: grid; grid-template-columns: minmax(240px, 34%) 1fr; height: 100%; min-height: 0; }
  .side { display: flex; flex-direction: column; min-height: 0; border-right: var(--border-width) solid var(--line); }
  .files { flex: 1; overflow: auto; min-height: 0; padding: 0 var(--sp-8) var(--sp-16) 0; }
  /* the header stays at the top, set apart from the tree */
  /* the header stays at the top, set apart from the tree; its columns are the rows' (box, then icon) */
  .files { container-type: inline-size; }
  /* under the files: the message and the button, apart from the list */
  .commit { flex: none; border-top: var(--border-width) solid var(--line); background: var(--panel); padding: var(--sp-12) var(--sp-14) var(--sp-14); }
  .commit :global(textarea) { width: 100%; resize: vertical; min-height: 54px; }
  .head { position: sticky; top: 0; z-index: var(--z-sticky); margin: 0 calc(var(--sp-8) * -1) var(--sp-6) 0; padding: var(--sp-8) var(--sp-8) var(--sp-6) 0;
    background: var(--panel); border-bottom: var(--border-width) solid var(--line); font-size: var(--fs-sm); color: var(--muted);
    display: grid; align-items: center; row-gap: var(--sp-4);
    grid-template-columns: 22px 20px minmax(0, 1fr) auto auto;
    grid-template-areas: "chev pick title views views" ". . total discard all"; }
  /* (no All files switch: the total takes its room) */
  .head.fixed { grid-template-areas: "chev pick title views views" ". . total total discard"; }
  @container (min-width: 380px) {
    .head, .head.fixed { grid-template-columns: 22px 20px auto minmax(0, 1fr) auto auto auto;
      grid-template-areas: "chev pick title total discard all views"; }
    .h-total { padding-left: var(--sp-10); }
    .h-revert { margin-right: var(--sp-10); }
  }
  .h-chev { grid-area: chev; margin-left: var(--sp-4); }
  .h-pick { grid-area: pick; }
  .h-title { grid-area: title; padding-left: var(--sp-4); text-transform: uppercase; letter-spacing: .06em;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .h-total { grid-area: total; padding-left: var(--sp-4); color: var(--faint); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .h-revert { grid-area: discard; justify-self: end; } /* (not "revert": a CSS keyword) */
  .h-all { grid-area: all; justify-self: end; }
  .h-views { grid-area: views; justify-self: end; margin-left: var(--sp-8); }
  /* List or tree: two small segments */
  .views { display: flex; border: var(--border-width) solid var(--line-strong); border-radius: var(--radius-pill); padding: 1px; }
  .views button { border: none; background: transparent; padding: 1px var(--sp-6); border-radius: var(--radius-pill);
    font-size: var(--fs-xs); color: var(--muted); text-transform: none; letter-spacing: 0; }
  .views button.on { background: var(--text); color: var(--bg); }
  .views button { display: inline-flex; align-items: center; }
  .views svg { width: 13px; height: 13px; }
  /* The list: a row per change, its folder under its name */
  li.two { height: 44px; }
  li.two .file { padding-top: var(--sp-4); padding-bottom: var(--sp-4); }
  /* (hovered or picked: the whole row, its box too) */
  li.two::before { content: ""; position: absolute; inset: 2px 0 2px 18px; border-radius: var(--radius); pointer-events: none; }
  li.two:hover::before { background: var(--panel); }
  li.two:has(.file.on)::before { background: var(--accent-soft); }
  li.two .file:hover, li.two .file.on { background: transparent; }
  li.two > :not(.more) { position: relative; }
  li.two .more { z-index: 1; }
  .names { display: flex; flex-direction: column; min-width: 0; line-height: 1.25; }
  .names .fname { font-weight: var(--fw-semibold); }
  .fdir { font-size: var(--fs-xs); color: var(--faint); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .revert { flex: none; display: inline-flex; padding: var(--sp-4); border-radius: var(--radius-sm); color: var(--muted); }
  .revert svg { width: 14px; height: 14px; }
  .revert:hover:not(:disabled) { color: var(--danger); background: var(--hover); }
  .revert:disabled { opacity: .35; }
  .all { display: flex; align-items: center; gap: var(--sp-4); margin: 0; text-transform: none; letter-spacing: 0; cursor: pointer; }
  /* iOS-style switch */
  .switch { appearance: none; position: relative; width: 26px; height: 15px; margin: 0; flex: none; cursor: pointer;
    border: none; padding: 0; border-radius: var(--radius-lg); background: var(--hover-strong); transition: background .15s; }
  .switch::after { content: ""; position: absolute; top: 2px; left: 2px; width: 11px; height: 11px; border-radius: 50%;
    background: var(--switch-knob); transition: transform .15s; }
  .switch:checked { background: var(--accent); }
  .switch:checked::after { transform: translateX(11px); background: #fff; }
  .switch:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  .empty { padding: 0 var(--sp-8); font-size: var(--fs-md); }
  ul { list-style: none; margin: 0; padding: 0; }
  li { position: relative; display: flex; align-items: center; height: 30px; }
  .indent { flex: none; }
  .chevbtn { flex: none; width: 18px; height: 22px; padding: 0; margin-left: var(--sp-4); display: flex; align-items: center;
    justify-content: center; border: none; background: transparent; }
  /* The commit box: ticked, unticked, or some of a folder (gray with a dash). */
  /* (padding 0: inputs have padding everywhere, which made the box wide) */
  .pick { appearance: none; position: relative; flex: none; width: 14px; min-width: 14px; height: 14px; padding: 0;
    margin: 0 var(--sp-4) 0 var(--sp-2); border: 1.5px solid var(--muted); border-radius: var(--radius-xs); background: transparent; cursor: pointer; }
  .pick:checked { background: var(--accent); border-color: var(--accent); }
  .pick:checked::after { content: ""; position: absolute; left: 3.5px; top: 0.5px; width: 3.5px; height: 7.5px;
    border: solid var(--accent-ink); border-width: 0 2px 2px 0; transform: rotate(45deg); }
  .pick:indeterminate { background: var(--dim); border-color: var(--dim); }
  .pick:indeterminate::after { content: ""; position: absolute; left: 2px; right: 2px; top: 4.5px; height: 2px;
    border-radius: 1px; background: #fff; }
  .pick:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
  span.pick { border-color: transparent; cursor: default; }
  .chev { width: 12px; height: 12px; flex: none; color: var(--muted); transition: transform .12s; }
  .chev.open { transform: rotate(90deg); }
  /* Folders read like files: bright when they hold changes, muted otherwise,
     faint when nothing inside is tracked. */
  .dir .fname { color: var(--muted); }
  .dir.changed .fname { color: var(--text); }
  .dir.untracked .fname { color: var(--faint); }
  .count { font-size: var(--fs-xs); padding: 0 var(--sp-6); border-radius: var(--radius-lg); background: var(--hover); color: var(--mod); }
  .file { flex: 1; min-width: 0; display: flex; align-items: center; gap: var(--sp-6); border: none; background: transparent;
    padding: var(--sp-4) var(--sp-6) var(--sp-4) var(--sp-4); border-radius: var(--radius); text-align: left; font-size: var(--fs-base); }
  .file:hover { background: var(--panel); }
  .file.on { background: var(--panel-2); }
  /* What changed, at the end of the row: a small colored square. */
  /* What changed at the far right; ⋯ (on hover) just left of it, in room kept for it. */
  .right { margin-left: auto; display: flex; align-items: center; justify-content: flex-end; gap: var(--sp-6); flex: none; min-width: 56px; }
  .right .sym, .right .count { margin-left: 22px; }
  .sym { width: 16px; height: 16px; border-radius: var(--radius-sm); display: inline-flex; align-items: center; justify-content: center;
    font-size: var(--fs-sm); font-weight: var(--fw-bold); line-height: 1; }
  .file.added .sym { background: var(--add-soft); }
  .file.deleted .sym { background: var(--del-soft); }
  .file.modified .sym { background: var(--mod-soft); }
  .file.added .sym { color: var(--add); }
  .file.deleted .sym { color: var(--del); }
  .file.modified .sym { color: var(--mod); }
  .file.deleted .fname { text-decoration: line-through; color: var(--muted); }
  .file.untracked .sym, .file.untracked .fname { color: var(--muted); }
  .file.ignored .fname, .file.unchanged .fname { color: var(--muted); }
  .file.ignored .fname { color: var(--faint); }
  .fname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .from { flex: 0 1000 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
    font-size: var(--fs-sm); color: var(--faint); }
  .file.renamed .sym { color: var(--warn); background: var(--warn-soft); }
  .vsym.renamed { color: var(--warn); }
  .live { font-size: var(--fs-xs); padding: 0 var(--sp-4); border-radius: var(--radius-pill); background: var(--hover); color: var(--muted);
    font-variant-numeric: tabular-nums; flex: none; }
  .live.odd { background: var(--warn-bg); color: var(--warn); }
  .more { position: absolute; right: 28px; top: 50%; transform: translateY(-50%); visibility: hidden; padding: 0 var(--sp-6); }
  li:hover .more { visibility: visible; }

  .detail { overflow: auto; min-height: 0; padding: var(--sp-12) var(--sp-4) var(--sp-16) var(--sp-20); }
  .detail-h { display: flex; flex-wrap: wrap; align-items: flex-start; gap: var(--sp-8) var(--sp-12); margin-bottom: var(--sp-10); }
  .title { flex: 1 1 180px; min-width: 0; }
  .dname { display: flex; align-items: center; gap: var(--sp-6); font-weight: var(--fw-semibold); font-size: var(--fs-lg); }
  .small { font-size: var(--fs-sm); }
  .modes { display: flex; }
  .modes button { padding: var(--sp-4) var(--sp-10); font-size: var(--fs-md); border-radius: 0; }
  .modes button:first-child { border-radius: var(--radius) 0 0 var(--radius); }
  .modes button:last-child { border-radius: 0 var(--radius) var(--radius) 0; margin-left: -1px; }
  .modes button.on { background: var(--accent); color: var(--accent-ink); border-color: var(--accent); }
  .lines { padding: var(--sp-8) var(--sp-10); background: var(--bg); border: var(--border-width) solid var(--line); border-radius: var(--radius); line-height: 1.6; user-select: text; }
  .add { color: var(--add); }
  .del { color: var(--del); }
  .mod { color: var(--mod); }
  .versions { display: flex; flex-direction: column; gap: var(--sp-2); margin-bottom: var(--sp-14); }
  .versions button { width: 100%; display: flex; align-items: center; gap: var(--sp-8); border: none; background: transparent;
    padding: var(--sp-6) var(--sp-8); border-radius: var(--radius); text-align: left; }
  .versions button:hover { background: var(--panel); }
  .versions button.on { background: var(--panel-2); }
  .vsym { width: 12px; text-align: center; font-weight: var(--fw-bold); }
  .vsym.added { color: var(--add); }
  .vsym.deleted { color: var(--del); }
  .vsym.modified { color: var(--mod); }
  .vmsg { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .picked { border-top: var(--border-width) solid var(--line); padding-top: var(--sp-14); }
  .restore { display: flex; align-items: center; gap: var(--sp-10); margin-bottom: var(--sp-14); }
  .restore button { padding: var(--sp-4) var(--sp-12); font-size: var(--fs-md); }

  .ctx { position: fixed; z-index: var(--z-submenu); min-width: 210px; padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop); }
  .ctx .item { display: block; width: 100%; border: none; background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left; }
  .ctx .item:hover:not(:disabled) { background: var(--hover); }
  .sub { position: relative; }
  .has-sub { display: flex !important; align-items: center; }
  .arrow { margin-left: auto; color: var(--muted); }
  .submenu { position: absolute; left: calc(100% + 2px); top: -6px; min-width: 250px; }
  .submenu.left { left: auto; right: calc(100% + 2px); }
  .submenu .item { display: flex; flex-direction: column; gap: 1px; }
  .pat { font-size: var(--fs-xs); }
  .note { margin: var(--sp-6) var(--sp-8) var(--sp-2); font-size: var(--fs-xs); line-height: 1.4; }
  .danger-text { color: var(--danger); }
  .sep { height: 1px; background: var(--line); margin: var(--sp-6) 0; }
</style>
