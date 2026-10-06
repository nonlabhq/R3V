<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { formatBytes, previewURL, type ProjectFile } from "./api";
  import FileIcon from "./FileIcon.svelte";

  // The Files tab's list, like a file explorer: one folder at a time (the
  // path above goes back up), as a list with columns or as a grid, sorted
  // and filtered by kind of file. How it looks is remembered.
  let { root, files, selected, onselect, onmenu }: {
    root: string;
    files: ProjectFile[];
    selected: string;
    onselect: (path: string) => void;
    onmenu?: (e: MouseEvent, path: string, dir: boolean) => void;
  } = $props();

  type SortKey = "name" | "modified" | "size" | "type";
  type Look = { sort: SortKey; desc: boolean; mode: "list" | "grid"; cols: { modified: boolean; size: boolean; type: boolean } };
  const KEY = "r3v.explorer";
  const look = $state<Look>((() => {
    const base: Look = { sort: "name", desc: false, mode: "list", cols: { modified: true, size: true, type: true } };
    try { return { ...base, ...JSON.parse(localStorage.getItem(KEY) ?? "{}") }; } catch { return base; }
  })());
  $effect(() => {
    const s = JSON.stringify(look);
    try { localStorage.setItem(KEY, s); } catch { /* not remembered */ }
  });

  let cwd = $state(""); // the folder shown ("" the project folder)
  let kinds = $state<string[]>([]); // the kinds shown (none: all)
  let menu = $state<"" | "sort" | "filter" | "cols">("");

  const kindName = (k: string) => (({
    set: t("Live Set"), live: t("Live clip, preset or rack"), audio: t("Audio"), midi: "MIDI", other: t("File"),
    scene: t("Scene"), level: t("Level"), prefab: "Prefab", asset: t("Asset"), script: t("Code"), image: t("Image"),
    model: t("3D model"), meta: "Unity .meta",
  }) as Record<string, string>)[k] ?? t("File");
  const ext = (p: string) => (p.includes(".") ? p.slice(p.lastIndexOf(".") + 1).toUpperCase() : "");
  const name = (p: string) => p.slice(p.lastIndexOf("/") + 1);
  const when = (iso: string) => (iso ? new Date(iso).toLocaleString(undefined, { dateStyle: "short", timeStyle: "short" }) : "");

  let allKinds = $derived([...new Set(files.map((f) => f.kind))].sort((a, b) => kindName(a).localeCompare(kindName(b))));
  let shown = $derived(kinds.length ? files.filter((f) => kinds.includes(f.kind)) : files);

  // The folder's entries: its folders (with what's in them), then its files.
  type Entry = { dir: boolean; path: string; name: string; kind: string; size: number; modified: string; count: number; file?: ProjectFile };
  let entries = $derived.by(() => {
    const prefix = cwd ? cwd + "/" : "";
    const dirs = new Map<string, Entry>();
    const out: Entry[] = [];
    for (const f of shown) {
      if (!f.path.startsWith(prefix)) continue;
      const rest = f.path.slice(prefix.length);
      const slash = rest.indexOf("/");
      if (slash < 0) {
        out.push({ dir: false, path: f.path, name: rest, kind: f.kind, size: f.size, modified: f.modified, count: 0, file: f });
        continue;
      }
      const d = rest.slice(0, slash);
      const e = dirs.get(d) ?? { dir: true, path: prefix + d, name: d, kind: "folder", size: 0, modified: "", count: 0 };
      e.size += f.size;
      e.count++;
      if (f.modified > e.modified) e.modified = f.modified;
      dirs.set(d, e);
    }
    const by = (a: Entry, b: Entry) => {
      const r = look.sort === "modified" ? a.modified.localeCompare(b.modified)
        : look.sort === "size" ? a.size - b.size
        : look.sort === "type" ? (a.dir ? "" : kindName(a.kind) + ext(a.path)).localeCompare(b.dir ? "" : kindName(b.kind) + ext(b.path))
        : 0;
      return (r || a.name.localeCompare(b.name, undefined, { numeric: true })) * (look.desc ? -1 : 1);
    };
    return [...[...dirs.values()].sort(by), ...out.sort(by)];
  });
  // The path above the list.
  let crumbs = $derived(cwd ? cwd.split("/").map((n, i, all) => ({ name: n, path: all.slice(0, i + 1).join("/") })) : []);
  // A folder that is gone (or filtered away): back up to one that is there.
  $effect(() => {
    if (cwd && !shown.some((f) => f.path.startsWith(cwd + "/"))) cwd = cwd.includes("/") ? cwd.slice(0, cwd.lastIndexOf("/")) : "";
  });

  // (a double click's second click lands on the folder entered by the first:
  // only the first opens)
  function open(e: Entry, ev?: MouseEvent) {
    if (ev && ev.detail > 1) return;
    if (e.dir) cwd = e.path;
    else onselect(e.path);
  }
  function sortBy(k: SortKey) {
    if (look.sort === k) look.desc = !look.desc;
    else { look.sort = k; look.desc = k === "modified" || k === "size"; }
  }
  function toggleKind(k: string) {
    kinds = kinds.includes(k) ? kinds.filter((x) => x !== k) : [...kinds, k];
  }
  const sortNames = (): Record<SortKey, string> => ({ name: t("Name"), modified: t("Date modified"), size: t("Size"), type: t("Type") });
  const sym: Record<string, string> = { added: "A", modified: "M", deleted: "D", renamed: "R" };
  let cols = $derived(["minmax(160px, 1fr)", look.cols.modified ? "130px" : "", look.cols.size ? "76px" : "", look.cols.type ? "110px" : ""]
    .filter(Boolean).join(" "));
  let rowMin = $derived(160 + (look.cols.modified ? 140 : 0) + (look.cols.size ? 86 : 0) + (look.cols.type ? 120 : 0) + 16);
</script>

<svelte:window onclick={(e) => { if (menu && !(e.target as HTMLElement).closest(".tool-wrap")) menu = ""; }} />

<div class="explorer">
  <div class="toolbar">
    <nav class="crumbs" aria-label={t("Folder")}>
      <button class:on={!cwd} onclick={() => (cwd = "")}>{t("Project")}</button>
      {#each crumbs as c (c.path)}
        <span class="slash">/</span><button class:on={c.path === cwd} onclick={() => (cwd = c.path)}>{c.name}</button>
      {/each}
    </nav>
    <div class="tools">
      <div class="tool-wrap">
        <button class="tool" onclick={() => (menu = menu === "sort" ? "" : "sort")} title={t("Sort")}>
          ⇅ {sortNames()[look.sort]}
        </button>
        {#if menu === "sort"}
          <div class="menu surface-menu" role="menu">
            {#each Object.entries(sortNames()) as [k, label] (k)}
              <button class="item" class:on={look.sort === k} onclick={() => sortBy(k as SortKey)}>
                {label}{#if look.sort === k}<span class="faint">{look.desc ? "↓" : "↑"}</span>{/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>
      <div class="tool-wrap">
        <button class="tool" class:active={kinds.length > 0} onclick={() => (menu = menu === "filter" ? "" : "filter")} title={t("Show only some kinds of file")}>
          ⏷ {kinds.length ? tn(kinds.length, "{count} kind", "{count} kinds", { count: kinds.length }) : t("All kinds")}
        </button>
        {#if menu === "filter"}
          <div class="menu surface-menu" role="menu">
            {#each allKinds as k (k)}
              <label class="item check"><input type="checkbox" checked={kinds.includes(k)} onchange={() => toggleKind(k)} />
                <FileIcon kind={k} /> {kindName(k)}</label>
            {/each}
            {#if kinds.length}<button class="item" onclick={() => (kinds = [])}>{t("Show all")}</button>{/if}
          </div>
        {/if}
      </div>
      {#if look.mode === "list"}
        <div class="tool-wrap">
          <button class="tool" onclick={() => (menu = menu === "cols" ? "" : "cols")} title={t("Columns")}>▥</button>
          {#if menu === "cols"}
            <div class="menu right surface-menu" role="menu">
              <label class="item check"><input type="checkbox" bind:checked={look.cols.modified} /> {t("Date modified")}</label>
              <label class="item check"><input type="checkbox" bind:checked={look.cols.size} /> {t("Size")}</label>
              <label class="item check"><input type="checkbox" bind:checked={look.cols.type} /> {t("Type")}</label>
            </div>
          {/if}
        </div>
      {/if}
      <div class="modes" role="group" aria-label={t("Display")}>
        <button class:on={look.mode === "list"} onclick={() => (look.mode = "list")} title={t("List")} aria-label={t("List")}>☰</button>
        <button class:on={look.mode === "grid"} onclick={() => (look.mode = "grid")} title={t("Grid")} aria-label={t("Grid")}>▦</button>
      </div>
    </div>
  </div>

  {#if look.mode === "list"}
    <div class="list" role="grid" aria-label={t("Files")}>
      <div class="row head" role="row" style:grid-template-columns={cols} style:min-width="{rowMin}px">
        <button role="columnheader" onclick={() => sortBy("name")}>{t("Name")}{#if look.sort === "name"} {look.desc ? "↓" : "↑"}{/if}</button>
        {#if look.cols.modified}<button role="columnheader" onclick={() => sortBy("modified")}>{t("Date modified")}{#if look.sort === "modified"} {look.desc ? "↓" : "↑"}{/if}</button>{/if}
        {#if look.cols.size}<button role="columnheader" class="num" onclick={() => sortBy("size")}>{t("Size")}{#if look.sort === "size"} {look.desc ? "↓" : "↑"}{/if}</button>{/if}
        {#if look.cols.type}<button role="columnheader" onclick={() => sortBy("type")}>{t("Type")}{#if look.sort === "type"} {look.desc ? "↓" : "↑"}{/if}</button>{/if}
      </div>
      {#each entries as e (e.path)}
        <button class="row" role="row" class:on={!e.dir && e.path === selected} class:gone={e.file?.status === "deleted"}
          style:grid-template-columns={cols} style:min-width="{rowMin}px" onclick={(ev) => open(e, ev)}
          oncontextmenu={(ev) => { ev.preventDefault(); onmenu?.(ev, e.path, e.dir); }} title={e.path}>
          <span class="cell name"><FileIcon kind={e.kind} open={false} /><span class="nm">{e.name}</span>
            {#if e.file && sym[e.file.status]}<span class="st {e.file.status}">{sym[e.file.status]}</span>{/if}</span>
          {#if look.cols.modified}<span class="cell faint">{when(e.modified)}</span>{/if}
          {#if look.cols.size}<span class="cell num faint">{e.dir ? tn(e.count, "{count} file", "{count} files", { count: e.count }) : formatBytes(e.size)}</span>{/if}
          {#if look.cols.type}<span class="cell faint">{e.dir ? t("Folder") : `${kindName(e.kind)}${ext(e.path) ? ` · ${ext(e.path)}` : ""}`}</span>{/if}
        </button>
      {:else}
        <p class="muted empty">{kinds.length ? t("No files of these kinds here.") : t("The project folder is empty.")}</p>
      {/each}
    </div>
  {:else}
    <div class="grid" aria-label={t("Files")}>
      {#each entries as e (e.path)}
        <button class="tile" class:on={!e.dir && e.path === selected} class:gone={e.file?.status === "deleted"}
          onclick={(ev) => open(e, ev)} oncontextmenu={(ev) => { ev.preventDefault(); onmenu?.(ev, e.path, e.dir); }} title={e.path}>
          <span class="thumb">
            {#if e.file?.preview && e.file.status !== "deleted"}
              <img src="{previewURL(root, e.path, '', 256)}&t={e.modified}" alt="" loading="lazy" />
            {:else}
              <span class="big-icon"><FileIcon kind={e.kind} /></span>
            {/if}
            {#if e.file && sym[e.file.status]}<span class="st {e.file.status}">{sym[e.file.status]}</span>{/if}
          </span>
          <span class="tname">{e.name}</span>
        </button>
      {:else}
        <p class="muted empty">{kinds.length ? t("No files of these kinds here.") : t("The project folder is empty.")}</p>
      {/each}
    </div>
  {/if}
</div>

<style>
  .explorer { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .toolbar { flex: none; display: flex; align-items: center; gap: var(--sp-8); padding: var(--sp-8) var(--sp-10);
    border-bottom: var(--border-width) solid var(--line); flex-wrap: wrap; }
  .crumbs { flex: 1; min-width: 0; display: flex; align-items: center; gap: var(--sp-2); overflow: hidden; white-space: nowrap; }
  .crumbs button { border: none; background: transparent; padding: var(--sp-2) var(--sp-6); color: var(--muted); font-size: var(--fs-md); }
  .crumbs button.on { color: var(--text); font-weight: var(--fw-semibold); }
  .slash { color: var(--faint); }
  .tools { display: flex; align-items: center; gap: var(--sp-4); }
  .tool-wrap { position: relative; }
  .tool { padding: var(--sp-4) var(--sp-8); font-size: var(--fs-sm); }
  .tool.active { border-color: var(--accent); color: var(--accent); }
  .modes { display: flex; }
  .modes button { padding: var(--sp-4) var(--sp-8); font-size: var(--fs-sm); border-radius: 0; color: var(--muted); }
  .modes button:first-child { border-radius: var(--radius) 0 0 var(--radius); }
  .modes button:last-child { border-radius: 0 var(--radius) var(--radius) 0; border-left: none; }
  .modes button.on { color: var(--text); background: var(--hover); }
  .menu { position: absolute; top: calc(100% + 4px); left: 0; z-index: var(--z-dropdown); min-width: 180px; padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop); }
  .menu.right { left: auto; right: 0; }
  .item { display: flex; align-items: center; justify-content: space-between; gap: var(--sp-8); width: 100%; border: none;
    background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left; margin: 0; color: var(--text); font-size: var(--fs-md); }
  .item.check { justify-content: flex-start; cursor: pointer; }
  .item.check input { width: auto; margin: 0; }
  .item:hover { background: var(--hover); }
  .item.on { color: var(--accent); }
  .list { flex: 1; overflow: auto; min-height: 0; padding: 0 var(--sp-6) var(--sp-16); }
  .row { display: grid; align-items: center; gap: var(--sp-10); width: 100%; height: 30px; border: none; background: transparent;
    padding: 0 var(--sp-8); text-align: left; border-radius: var(--radius); font-size: var(--fs-md); }
  .row:hover:not(.head):not(.on) { background: var(--panel); }
  .row.on { background: var(--panel-2); }
  .row.gone .nm { color: var(--faint); text-decoration: line-through; }
  .row.head { position: sticky; top: 0; z-index: 1; background: var(--bg); border-radius: 0; height: 28px;
    border-bottom: var(--border-width) solid var(--line); margin-bottom: var(--sp-4); }
  .row.head button { border: none; background: transparent; padding: 0; text-align: left; font-size: var(--fs-xs);
    color: var(--faint); text-transform: uppercase; letter-spacing: .05em; }
  .row.head button:hover:not(:disabled) { color: var(--text); background: transparent; }
  .cell { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: var(--fs-sm); }
  .num { text-align: right; }
  .name { display: flex; align-items: center; gap: var(--sp-8); font-size: var(--fs-md); }
  .nm { overflow: hidden; text-overflow: ellipsis; }
  .st { flex: none; font-size: var(--fs-2xs); font-weight: var(--fw-bold); padding: 0 var(--sp-4); border-radius: var(--radius-sm);
    background: var(--mod-soft); color: var(--mod); }
  .st.added { background: var(--add-soft); color: var(--add); }
  .st.deleted { background: var(--del-soft); color: var(--del); }
  .grid { flex: 1; overflow: auto; min-height: 0; padding: var(--sp-12); display: grid;
    grid-template-columns: repeat(auto-fill, minmax(104px, 1fr)); gap: var(--sp-8); align-content: start; }
  .tile { display: flex; flex-direction: column; align-items: center; gap: var(--sp-6); padding: var(--sp-8); border: none;
    background: transparent; border-radius: var(--radius-lg); min-width: 0; }
  .tile:hover:not(.on) { background: var(--panel); }
  .tile.on { background: var(--panel-2); }
  .tile.gone .tname { color: var(--faint); text-decoration: line-through; }
  .thumb { position: relative; width: 72px; height: 72px; display: flex; align-items: center; justify-content: center;
    border-radius: var(--radius); overflow: hidden; }
  .thumb img { max-width: 100%; max-height: 100%; object-fit: contain; }
  .thumb .st { position: absolute; right: 2px; bottom: 2px; }
  .big-icon :global(svg) { width: 44px; height: 44px; }
  .tname { font-size: var(--fs-sm); max-width: 100%; text-align: center; overflow: hidden; display: -webkit-box;
    -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; word-break: break-word; }
  .empty { padding: var(--sp-16); }
</style>
