<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { Events } from "@wailsio/runtime";
  import { api, errorText, previewURL, type MemberLook, type ProjectFile, type State, type Version } from "./api";
  import type { IgnoreOption } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";
  import { toast } from "./notify.svelte";
  import { onDroppedFiles, clearDropMarks } from "./filedrop";
  import FilePanel from "./FilePanel.svelte";
  import ConvertDialog from "./ConvertDialog.svelte";
  import { portal } from "$shared/portal";
  import FileBrowser from "$shared/FileBrowser.svelte";
  import { baseName, dirOf, type Selection } from "$shared/files";
  import LockBadge from "./LockBadge.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import { breakLock, holderName, lockAt, locksOf, lockPaths, unlockPaths, type LockItem } from "./locks.svelte";
  import { branchLabel } from "$shared/branches";


  // The Files tab: the shared explorer ($shared/FileBrowser: folders, the
  // folder's files, the one picked) with what only the App does: the file's
  // panel and history, opening it in its program, renaming (F2), dragging
  // files out to other programs and copying in those dropped from Explorer,
  // locks, its menu (convert, ignore, discard).
  let { root, st, looks, onrestore, ondiscard, onrules }: {
    root: string;
    st: State;
    looks?: Record<string, MemberLook | undefined>; // members' colours and pictures, by id
    onrestore: (path: string, version: string, label: string, source: string) => void;
    ondiscard: (path: string) => void;
    onrules?: () => void; // .r3v.yaml changed (something left out)
  } = $props();

  let files = $state<ProjectFile[]>([]);
  let last = $state<Record<string, Version | undefined>>({});
  // File locks (off: nothing shows): a folder's own lock, a file's (its
  // own or a folder's over it).
  let lv = $derived(locksOf(root));
  const folderLock = (p: string) => (lv?.on ? lv.items.find((l) => l.prefix && l.path === p + "/") : undefined);
  const fileLock = (p: string) => lockAt(lv, p);
  let breaking = $state<{ path: string; name: string; agent: boolean } | null>(null);
  // An agent's lock names its branch as the team calls it.
  const branchName = (key: string) => branchLabel(st.branches, key);
  // Breaking a lock: asked first, but your own agent lock (taken from the
  // command line) is simply freed.
  function askBreak(l: LockItem) {
    menu = null;
    if (l.agent && l.mine) breakLock(root, l.path, true);
    else breaking = { path: l.path, name: holderName(l, branchName), agent: l.agent };
  }
  // Lock or unlock from the menu: the files picked with the one clicked.
  const lockTargets = (m: { path: string; dir: boolean }) =>
    m.dir ? [m.path + "/"] : sel.picked.includes(m.path) ? sel.picked.filter((p) => !fileLock(p)) : [m.path];
  function lockFromMenu(m: { path: string; dir: boolean }) { menu = null; lockPaths(root, lockTargets(m)); }
  function unlockFromMenu(path: string) { menu = null; unlockPaths(root, [path]); }
  let loadedAt = $state(0);
  // Until the files are first read: said so (an agent committing holds the
  // project a while; reading waits for it), not "no files".
  let slow = $state(false);
  $effect(() => {
    if (loadedAt) return;
    const id = setTimeout(() => (slow = true), 3000);
    return () => clearTimeout(id);
  });
  function loadFiles() {
    return api.ProjectFiles(root, true).then((f) => { files = f ?? []; loadedAt = Date.now(); })
      .catch((e) => toast(errorText(e), "error"));
  }
  $effect(() => {
    st; // read again with the project's state
    loadFiles();
  });
  $effect(() => {
    const head = st.head;
    if (!head) { last = {}; return; }
    api.LastChanges(root).then((l) => { if (st.head === head) last = l ?? {}; }).catch(() => {});
  });

  // Remembered per project: the folder shown, the look (list or grid).

  // What the explorer shows: the folder, the files picked (kept here too:
  // a copy, a rename, a conversion picks what it made).
  let dir = $state("");
  let sel = $state<Selection>({ picked: [], anchor: "" });
  let renameAsk = $state<{ path: string; dir: boolean } | null>(null);
  function pickDir(d: string) {
    if (d !== dir) { dir = d; sel = { picked: [], anchor: "" }; }
  }
  const pickFiles = (paths: string[]) => (sel = { picked: paths, anchor: paths[0] ?? "" });
  async function rename(path: string, isDir: boolean, name: string) {
    try {
      const to = await api.RenameFile(root, path, name);
      if (isDir) {
        const moved = (p: string) => (p === path || p.startsWith(path + "/") ? to + p.slice(path.length) : p);
        dir = moved(dir);
      }
      await loadFiles();
      if (!isDir) pickFiles([to]);
    } catch (e) {
      toast(errorText(e), "error", 9000);
    }
  }
  async function dragOut(paths: string[]) {
    try {
      await api.StartDrag(root, paths);
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      clearDropMarks(); // (a drop elsewhere, in another program, never tells this page)
    }
  }
  // A thumbnail, made again when the file changes.
  const thumb = (f: ProjectFile) => `${previewURL(root, f.path, "", 320)}&t=${f.modified}`;

  // --- files dropped from Explorer: copied into the folder they land on ---
  function dropped(d: { files: string[]; root: string; dir: string }) {
    if (d.root !== root || !d.files?.length) return;
    // Files dragged out and let go over the folder they are in: nothing to do.
    if (d.files.every((p) => dirOf(relTo(p) ?? "\0") === (d.dir ?? ""))) return;
    copyIn(d.dir ?? "", d.files);
  }
  $effect(() => {
    const offPage = onDroppedFiles(dropped);
    // (the app's own route, where the page couldn't tell the folder)
    const off = Events.On("files-dropped", (ev: { data: { files: string[]; root: string; dir: string } }) => dropped(ev.data));
    return () => { offPage(); off(); };
  });
  // A path's place in the project (null: outside it).
  function relTo(abs: string): string | null {
    const norm = (p: string) => p.replace(/\\/g, "/").replace(/\/+$/, "");
    const r = norm(root), p = norm(abs);
    return p.toLowerCase().startsWith(r.toLowerCase() + "/") ? p.slice(r.length + 1) : null;
  }
  async function copyIn(into: string, paths: string[]) {
    try {
      const res = await api.CopyIntoProject(root, into, paths);
      if (!res) return;
      if (res.clashes.length) {
        toast(tn(res.clashes.length, "Nothing was copied: {names} is already in {folder}. Rename it first, or drop it into another folder.",
          "Nothing was copied: {names} are already in {folder}. Rename them first, or drop them into another folder.",
          { names: res.clashes.join(", "), folder: into || st.name }), "error", 10000);
        return;
      }
      toast(tn(res.copied.length, "Copied {n} item into {folder}", "Copied {n} items into {folder}", { folder: into || st.name }), "ok");
      await loadFiles();
      pickDir(into);
      pickFiles(res.copied.filter((p) => dirOf(p) === into));
    } catch (e) {
      toast(errorText(e), "error", 9000);
    }
  }


  // --- the menu of a file or folder (right click) ---
  let menu = $state<{ path: string; x: number; y: number; dir: boolean; ignore: IgnoreOption[] } | null>(null);
  let ignoreOpen = $state(false);
  let menuW = $state(220), menuH = $state(200);
  // Samples to convert: the one right-clicked, or every one picked with it
  // when they all are samples.
  let converting = $state<string[]>([]);
  const toConvert = (path: string) => {
    const picked = sel.picked.includes(path) ? sel.picked.map((p) => files.find((f) => f.path === p)) : [];
    return picked.length > 1 && picked.every((f) => f?.kind === "audio" && f.status !== "deleted") ? sel.picked : [path];
  };
  function openMenu(e: MouseEvent, path: string, isDir: boolean) {
    ignoreOpen = false;
    menu = { path, x: e.clientX, y: e.clientY, dir: isDir, ignore: [] };
    api.IgnoreOptions(path, isDir).then((o) => { if (menu?.path === path) menu = { ...menu, ignore: o ?? [] }; }).catch(() => {});
  }
  async function ignore(pattern: string) {
    menu = null;
    try {
      await api.AddIgnoreRule(root, pattern);
      toast(t("Left out of versions: {pattern} — a rule in .r3v.yaml; commit it to share it with the team", { pattern }), "ok", 7000);
      await loadFiles();
      onrules?.();
    } catch (e) {
      toast(errorText(e), "error", 9000);
    }
  }
  function openFile(f: ProjectFile) {
    menu = null;
    if (f.status === "deleted") return;
    api.OpenInLive(root, f.path).catch((e) => toast(errorText(e), "error"));
  }
  function showInFolder(p: string) {
    menu = null;
    api.ShowFile(root, p).catch((e) => toast(errorText(e), "error"));
  }

</script>

<svelte:window onclick={(e) => { if (menu && !(e.target as HTMLElement).closest(".ctx")) menu = null; }}
  onkeydown={(e) => { if (e.key === "Escape" && menu) menu = null; }} />

<FileBrowser {files} loaded={!!loadedAt} {slow} name={st.name} remember={root} {last} {looks} bind:dir bind:sel bind:renameAsk
  {thumb} onopen={openFile} onmenu={openMenu} onrename={rename} ondragout={dragOut} dropRoot={root}
  emptyText={t("No files in this folder. Drop files here to copy them in.")}>
  {#snippet badge(path, where)}
    {@const l = path.endsWith("/") ? folderLock(path.slice(0, -1)) : fileLock(path)}
    {#if l}<LockBadge lock={l} {looks} {branchName} size={where === "tree" ? 14 : undefined} />{/if}
  {/snippet}
  {#snippet panel(current, picked)}
    {#if current}
      <FilePanel {root} {st} file={current} stamp={loadedAt} {onrestore} lock={fileLock(current.path)} {looks} {branchName}
        onbreak={askBreak} />
    {:else}
      <p class="muted pick-note">{picked > 1 ? tn(picked, "{n} file picked", "{n} files picked") : t("Pick a file to see it and its history.")}</p>
    {/if}
  {/snippet}
</FileBrowser>

{#if menu}
  {@const m = menu}
  {@const f = m.dir ? undefined : files.find((x) => x.path === m.path)}
  <div class="ctx surface-menu" role="menu" use:portal bind:offsetWidth={menuW} bind:offsetHeight={menuH}
    style:left="{Math.max(8, Math.min(m.x, window.innerWidth - menuW - 8))}px" style:top="{Math.max(8, Math.min(m.y, window.innerHeight - menuH - 8))}px">
    {#if f && f.status !== "deleted"}
      <button class="item" onclick={() => openFile(f)}>{t("Open")}</button>
    {/if}
    {#if m.dir || (f && f.status !== "deleted")}
      <button class="item" onclick={() => showInFolder(m.path)}>{t("Show in folder")}</button>
      <button class="item" onclick={() => { renameAsk = { path: m.path, dir: m.dir }; menu = null; }}>{t("Rename")}<span class="faint key">F2</span></button>
    {/if}
    {#if f?.kind === "audio" && f.status !== "deleted"}
      {@const n = toConvert(m.path).length}
      <button class="item" onclick={() => { converting = toConvert(m.path); menu = null; }}>{n > 1 ? t("Convert {n} samples…", { n }) : t("Convert…")}</button>
    {/if}
    {#if lv?.on}
      {@const own = m.dir ? folderLock(m.path) : lv.items.find((l) => l.path === m.path)}
      {@const over = m.dir ? lockAt(lv, m.path + "/") : fileLock(m.path)}
      <div class="sep"></div>
      {#if own?.agent && own.person}
        <p class="faint note">{t("Your changes: agents leave it alone until you commit")}</p>
      {:else if own?.agent}
        <!-- (people always win: any agent lock can be broken) -->
        <button class="item" class:danger-text={!own.mine} onclick={() => askBreak(own)}>{own.mine ? (m.dir ? t("Unlock folder") : t("Unlock")) : t("Break lock…")}</button>
      {:else if own?.mine}
        <button class="item" onclick={() => unlockFromMenu(own.path)}>{m.dir ? t("Unlock folder") : t("Unlock")}</button>
      {:else if own && lv.admin}
        <button class="item danger-text" onclick={() => askBreak(own)}>{t("Break lock…")}</button>
      {:else if !over && lv.team}
        <button class="item" onclick={() => lockFromMenu(m)}>{m.dir ? t("Lock folder") : t("Lock")}</button>
      {:else if over && !over.mine}
        <p class="faint note">{t("Locked by {name}", { name: holderName(over, branchName) })}</p>
      {/if}
    {/if}
    {#if f && ["added", "modified", "deleted", "renamed"].includes(f.status)}
      <button class="item danger-text" onclick={() => { const p = m.path; menu = null; ondiscard(p); }}>{t("Discard changes…")}</button>
    {/if}
    {#if m.ignore.length}
      <div class="sep"></div>
      <div class="sub" role="none" onmouseenter={() => (ignoreOpen = true)} onmouseleave={() => (ignoreOpen = false)}>
        <button class="item has-sub" onclick={() => (ignoreOpen = !ignoreOpen)} aria-expanded={ignoreOpen}>{t("Ignore")}<span class="arrow">›</span></button>
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


{#if breaking}
  {@const b = breaking}
  <ConfirmDialog title={t("Break the lock?")} danger confirm={t("Break lock")}
    text={b.agent ? t("{name} holds {file}. Breaking the lock lets you and other agents change it; what {name} changed there may then need your decisions when its branch is merged.", { name: b.name, file: b.path })
      : t("{name} holds {file}. Breaking the lock lets others change it; any changes {name} hasn't shared may then clash with theirs.", { name: b.name, file: b.path })}
    onclose={() => (breaking = null)} onconfirm={() => { const p = b.path, agent = b.agent; breaking = null; breakLock(root, p, agent); }} />
{/if}

{#if converting.length}
  <ConvertDialog {root} files={converting} onclose={() => (converting = [])}
    ondone={async (made) => {
      converting = [];
      toast(made.length > 1 ? t("Converted {n} samples", { n: made.length }) : t("Converted to {file}", { file: baseName(made[0] ?? "") }), "ok");
      await loadFiles();
      if (made.length) { pickDir(dirOf(made[0])); pickFiles(made.filter((p) => dirOf(p) === dirOf(made[0]))); }
    }} />
{/if}


<style>
  .pick-note { padding: var(--sp-20); }

  .ctx { position: fixed; z-index: var(--z-submenu); min-width: 210px; padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop); }
  .ctx .item { display: flex; align-items: center; width: 100%; border: none; background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left; }
  .ctx .item:hover:not(:disabled) { background: var(--hover); }
  .key { margin-left: auto; font-size: var(--fs-xs); }
  .ctx .sep { height: 1px; background: var(--line); margin: var(--sp-6) 0; }
  .sub { position: relative; }
  .arrow { margin-left: auto; color: var(--muted); }
  .submenu { position: absolute; left: calc(100% + 2px); top: -6px; min-width: 250px; }
  .submenu.left { left: auto; right: calc(100% + 2px); }
  .submenu .item { flex-direction: column; align-items: flex-start; gap: 1px; }
  .pat { font-size: var(--fs-xs); }
  .ctx .note { margin: var(--sp-6) var(--sp-8) var(--sp-2); font-size: var(--fs-xs); line-height: 1.4; }
  .danger-text { color: var(--danger); }
</style>