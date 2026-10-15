<script lang="ts">
  import { t, tn } from "./i18n";
  import ChangeMark from "./ChangeMark.svelte";
  import FileIcon from "./FileIcon.svelte";
  import Splitter from "./Splitter.svelte";
  import { splitPx } from "./splits.svelte";
  import { navKey, ownKey } from "./keynav";
  import { changesView, pickChangesView } from "./changesview.svelte";
  import type { ChangedFile } from "./types";
  import type { Snippet } from "svelte";

  // A version's files (the App's version details, R3V Cloud's): the files
  // it changed, as a list (each with its folder under its name) or a tree of
  // folders, and the picked one shown on the right by the viewer snippet
  // (each side gives its own: see changeSides). ↑ ↓ Home End move through
  // the files; the list's width is the "list" split.
  let { files, error = "", remember, picked = $bindable(""), viewer }: {
    files: ChangedFile[] | null; // null: being read
    error?: string;
    remember: string;            // where the list or tree choice is kept (the project)
    picked?: string;             // the file shown (its path)
    viewer: Snippet<[ChangedFile]>;
  } = $props();

  // The first file, once they are read (none picked, or the picked one gone).
  $effect(() => {
    if (files && !files.some((f) => f.path === picked)) picked = files[0]?.path ?? "";
  });
  let current = $derived(files?.find((f) => f.path === picked));

  let view = $derived(changesView(remember, files?.length ?? 0));
  type Folder = { path: string; name: string; folders: Map<string, Folder>; files: ChangedFile[]; count: number };
  let closed = $state<Record<string, boolean>>({});
  type Row = { folder?: Folder; file?: ChangedFile; depth: number };
  let rows = $derived.by((): Row[] => {
    const list = [...(files ?? [])].sort((a, b) => a.path.localeCompare(b.path));
    if (view === "list") return list.map((f) => ({ file: f, depth: 0 }));
    const mk = (path: string, name: string): Folder => ({ path, name, folders: new Map(), files: [], count: 0 });
    const top = mk("", "");
    for (const f of list) {
      const parts = f.path.split("/");
      let node = top;
      for (let k = 0; k < parts.length - 1; k++) {
        if (!node.folders.has(parts[k])) node.folders.set(parts[k], mk(parts.slice(0, k + 1).join("/"), parts[k]));
        node = node.folders.get(parts[k])!;
        node.count++;
      }
      node.files.push(f);
    }
    const out: Row[] = [];
    const walk = (node: Folder, depth: number) => {
      for (const sub of [...node.folders.values()].sort((a, b) => a.name.localeCompare(b.name))) {
        out.push({ folder: sub, depth });
        if (!closed[sub.path]) walk(sub, depth + 1);
      }
      for (const f of node.files) out.push({ file: f, depth });
    };
    walk(top, 0);
    return out;
  });
  let shownFiles = $derived(rows.filter((r) => r.file).map((r) => r.file!));
  let aside = $state<HTMLElement>();
  function onKey(e: KeyboardEvent) {
    if (!ownKey(e) || !files?.length) return;
    const nav = navKey(shownFiles.map((f) => ({ key: f.path })), picked, e.key, 10);
    if (!nav || !("to" in nav)) return;
    e.preventDefault();
    picked = nav.to;
    const to = nav.to;
    queueMicrotask(() => [...(aside?.querySelectorAll<HTMLElement>("[data-path]") ?? [])].find((x) => x.dataset.path === to)?.focus());
  }
  let bodyWidth = $state(0);
  let listWidth = $derived(splitPx("list", 0.34, bodyWidth, 240, 300));
  const name = (p: string) => p.slice(p.lastIndexOf("/") + 1);
  const dir = (p: string) => p.slice(0, p.lastIndexOf("/") + 1);
  const statusName = (s: string) => ({ added: t("New"), modified: t("Changed"), deleted: t("Deleted"),
    renamed: t("Moved"), untracked: t("No longer tracked") } as Record<string, string>)[s] ?? s;
</script>

<div class="body" bind:clientWidth={bodyWidth} style:grid-template-columns="{listWidth}px 1fr">
  {#if bodyWidth}<Splitter key="list" def={0.34} width={bodyWidth} minLeft={240} minRight={300} />{/if}
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <aside bind:this={aside} onkeydown={onKey}>
    <div class="list-h">
      <span>{files ? tn(files.length, "{count} file changed", "{count} files changed", { count: files.length }) : t("Changes")}</span>
      {#if files?.length}
        <div class="views" role="group" aria-label={t("Show the changes as")}>
          <button class:on={view === "list"} aria-pressed={view === "list"} onclick={() => pickChangesView(remember, "list")} title={t("List")} aria-label={t("List")}><svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 4h10M3 8h10M3 12h10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" fill="none" /></svg></button>
          <button class:on={view === "tree"} aria-pressed={view === "tree"} onclick={() => pickChangesView(remember, "tree")} title={t("Tree")} aria-label={t("Tree")}><svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 3.5h4M6 8h7M6 12.5h7M4 3.5v9h2M4 8h2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" /></svg></button>
        </div>
      {/if}
    </div>
    {#if error}
      <p class="error">{error}</p>
    {:else if files === null}
      <p class="muted pad">{t("Reading…")}</p>
    {:else if files.length === 0}
      <p class="muted pad">{t("No file changes.")}</p>
    {:else}
      <ul>
        {#each rows as row (row.file ? row.file.path : "dir:" + row.folder!.path)}
          {#if row.folder}
            {@const d = row.folder}
            <li style:padding-left="{row.depth * 14}px">
              <button class="folder" onclick={() => (closed[d.path] = !closed[d.path])} aria-expanded={!closed[d.path]} title={d.path}>
                <svg class="chev" class:open={!closed[d.path]} viewBox="0 0 10 10" aria-hidden="true"><path d="M3 1.5 L7 5 L3 8.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg>
                <FileIcon kind="folder" open={!closed[d.path]} />
                <span class="names"><span class="fname">{d.name}</span></span>
                <span class="count faint">{d.count}</span>
              </button>
            </li>
          {:else}
            {@const f = row.file!}
            <li style:padding-left="{row.depth * 14}px">
              <button class:on={f.path === picked} class:tree={view === "tree"} onclick={() => (picked = f.path)} title={f.path} data-path={f.path}>
                <FileIcon path={f.path} kind={f.kind} />
                <span class="names"><span class="fname">{name(f.path)}</span>{#if view === "list" && dir(f.path)}<span class="fdir">{dir(f.path)}</span>{/if}</span>
                <ChangeMark status={f.status} title={statusName(f.status)} />
              </button>
            </li>
          {/if}
        {/each}
      </ul>
    {/if}
  </aside>
  <section class="diff">
    {#if current}
      <div class="diff-h">
        <div class="dname"><FileIcon path={current.path} kind={current.kind} /> {name(current.path)}</div>
        <div class="faint small">{current.status === "renamed" ? t("Moved from {path}", { path: current.from ?? "" }) : statusName(current.status)}</div>
      </div>
      {@render viewer(current)}
    {:else if files && files.length}
      <p class="muted">{t("Pick a file on the left to see what changed.")}</p>
    {/if}
  </section>
</div>

<style>
  .body { position: relative; flex: 1; min-height: 0; display: grid; grid-template-columns: 270px 1fr; }
  aside { border-right: var(--border-width) solid var(--line); overflow: auto; padding: var(--sp-8); }
  .list-h { display: flex; align-items: center; justify-content: space-between; gap: var(--sp-8); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); padding: var(--sp-4) var(--sp-6) var(--sp-8); }
  .pad { padding: 0 var(--sp-6); }
  ul { list-style: none; margin: 0; padding: 0; }
  li button { display: flex; align-items: center; gap: var(--sp-8); width: 100%; border: none; background: transparent;
    padding: var(--sp-6) var(--sp-6); text-align: left; border-radius: var(--radius); }
  li button:hover:not(.on) { background: var(--panel); }
  li button.on { background: var(--panel-2); }
  .names { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .fname, .fdir { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .fname { font-size: var(--fs-md); }
  .fdir { font-size: var(--fs-xs); color: var(--faint); }
  .diff { overflow: auto; padding: var(--sp-14) var(--sp-16); min-width: 0; }
  .diff-h { margin-bottom: var(--sp-10); }
  .dname { display: flex; align-items: center; gap: var(--sp-6); font-weight: var(--fw-semibold); }
  .small { font-size: var(--fs-sm); }
  .error { color: var(--danger); padding: 0 var(--sp-6); }
  .views { display: flex; border: var(--border-width) solid var(--line-strong); border-radius: var(--radius-pill); padding: 1px; }
  .views button { border: none; background: transparent; padding: 1px var(--sp-6); border-radius: var(--radius-pill);
    font-size: var(--fs-xs); color: var(--muted); text-transform: none; letter-spacing: 0; width: auto; }
  .views button.on { background: var(--text); color: var(--bg); }
  .views button { display: inline-flex; align-items: center; }
  .views svg { width: 13px; height: 13px; }
  li button.folder { padding-left: var(--sp-2); color: var(--muted); }
  li button.tree { padding-left: calc(var(--sp-6) + 14px); }
  .chev { width: 10px; height: 10px; flex: none; transition: transform .12s; }
  .chev.open { transform: rotate(90deg); }
  .count { font-size: var(--fs-xs); }
</style>
