<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { Events } from "@wailsio/runtime";
  import { api, ago, errorText, formatBytes, previewURL, type MemberLook, type ProjectFile, type State, type Version } from "./api";
  import type { IgnoreOption } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";
  import { toast } from "./notify.svelte";
  import FileIcon from "./FileIcon.svelte";
  import Avatar from "./Avatar.svelte";
  import FilePanel from "./FilePanel.svelte";
  import ConvertDialog from "./ConvertDialog.svelte";
  import { portal } from "./portal";
  import { branchLabel, branchLane } from "./branches";
  import { gridKey, navKey, ownKey, type NavRow } from "./keynav";
  import { baseName, clickPick, defaultMode, dirOf, filesIn, findFolder, folderTree, isChanged, marqueePick, toolOf,
    type Folder, type Selection } from "./files";

  // The Files tab, file-centred (the Overview is version-centred): the
  // project's folders on the left (with how many changed files each holds),
  // the files of the folder picked in the middle (a list or a grid, each
  // folder remembering which), and the file picked on the right with its
  // history. Files are picked like in Explorer (Ctrl, Shift, a box drawn
  // around them), renamed with F2, dragged out to other programs, and
  // files dropped from Explorer are copied in.
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
  let loadedAt = $state(0);
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

  // Remembered per project: the folder shown, each folder's look.
  const remember = <T,>(key: string, base: T): T => {
    try { return { ...base, ...JSON.parse(localStorage.getItem(key) ?? "{}") }; } catch { return base; }
  };
  const keep = (key: string, value: unknown) => {
    try { localStorage.setItem(key, JSON.stringify(value)); } catch { /* not remembered */ }
  };
  // (the tab is made again for another project)
  // svelte-ignore state_referenced_locally
  let dir = $state(remember(`r3v.files:${root}`, { dir: "" }).dir);
  // svelte-ignore state_referenced_locally
  let modes = $state<Record<string, "list" | "grid">>(remember(`r3v.fileModes:${root}`, {}));
  $effect(() => keep(`r3v.files:${root}`, { dir }));
  $effect(() => keep(`r3v.fileModes:${root}`, modes));

  let tree = $derived(folderTree(files, st.name));
  // A folder that went away: back up to one that is there.
  $effect(() => {
    if (files.length && !findFolder(tree, dir)) {
      let d = dir;
      while (d && !findFolder(tree, d)) d = dirOf(d);
      dir = d;
    }
  });

  // --- the folder tree ---
  let open = $state<Record<string, boolean>>({});
  const isOpen = (p: string) => p === "" || (open[p] ?? (dir === p || dir.startsWith(p + "/")));
  type TreeRow = { f: Folder; depth: number };
  let treeRows = $derived.by(() => {
    const out: TreeRow[] = [];
    const walk = (f: Folder, depth: number) => {
      out.push({ f, depth });
      if (isOpen(f.path)) for (const s of f.folders) walk(s, depth + 1);
    };
    walk(tree, 0);
    return out;
  });
  function pickDir(p: string) {
    if (p !== dir) { dir = p; filter = ""; sel = { picked: [], anchor: "" }; }
  }
  function onTreeKey(e: KeyboardEvent) {
    if (e.key === "F2" && dir) { e.preventDefault(); startRename(dir, true); return; }
    if (!ownKey(e)) return;
    const rows: NavRow[] = treeRows.map((r) => ({ key: r.f.path, dir: true, open: r.f.folders.length > 0 && isOpen(r.f.path), depth: r.depth }));
    const nav = navKey(rows, dir, e.key);
    if (!nav) return;
    e.preventDefault();
    if ("toggle" in nav) open[nav.toggle] = !isOpen(nav.toggle);
    else pickDir(nav.to);
  }

  // --- the files of the folder ---
  let filter = $state("");
  let here = $derived(filesIn(files, dir));
  let mode = $derived(modes[dir] ?? defaultMode(here));
  type SortKey = "name" | "size" | "change";
  let sort = $state<{ key: SortKey; desc: boolean }>({ key: "change", desc: true });
  const changeTime = (f: ProjectFile) => last[f.path]?.time ?? (isChanged(f) ? "9" : "");
  let shown = $derived.by(() => {
    const q = filter.trim().toLowerCase();
    const list = q ? here.filter((f) => baseName(f.path).toLowerCase().includes(q)) : [...here];
    const name = (f: ProjectFile) => baseName(f.path);
    return list.sort((a, b) => {
      const r = sort.key === "size" ? a.size - b.size : sort.key === "change" ? changeTime(a).localeCompare(changeTime(b)) : 0;
      return (r || name(a).localeCompare(name(b), undefined, { numeric: true })) * (sort.desc ? -1 : 1);
    });
  });
  let order = $derived(shown.map((f) => f.path));
  let hereSize = $derived(here.reduce((n, f) => n + f.size, 0));
  function sortBy(k: SortKey) {
    if (sort.key === k) sort.desc = !sort.desc;
    else sort = { key: k, desc: k !== "name" };
  }
  let crumbs = $derived(dir ? dir.split("/").map((n, i, all) => ({ name: n, path: all.slice(0, i + 1).join("/") })) : []);

  // --- picking files ---
  let sel = $state<Selection>({ picked: [], anchor: "" });
  let clicked = $state(""); // the file last clicked or moved to: the one shown on the right
  let focus = $derived(sel.picked.includes(clicked) ? clicked : sel.picked[sel.picked.length - 1] ?? "");
  let current = $derived(files.find((f) => f.path === focus));
  // Files gone (renamed, deleted) drop out of what is picked.
  $effect(() => {
    const there = new Set(files.map((f) => f.path));
    if (sel.picked.some((p) => !there.has(p))) sel = { picked: sel.picked.filter((p) => there.has(p)), anchor: there.has(sel.anchor) ? sel.anchor : "" };
  });
  let area = $state<HTMLElement>();
  function pick(e: MouseEvent, path: string) {
    if (suppressClick) { suppressClick = false; return; }
    sel = clickPick(sel, order, path, { ctrl: e.ctrlKey || e.metaKey, shift: e.shiftKey });
    clicked = path;
    area?.focus({ preventScroll: true });
  }
  function onAreaKey(e: KeyboardEvent) {
    if (renaming) return;
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "a" && ownKeyTarget(e)) {
      e.preventDefault();
      sel = { picked: [...order], anchor: order[0] ?? "" };
      return;
    }
    if (e.key === "F2" && focus) { e.preventDefault(); startRename(focus, false); return; }
    if (e.key === "Escape" && sel.picked.length) { e.preventDefault(); sel = { picked: [], anchor: "" }; return; }
    if (!ownKey(e)) return;
    if (e.key === "Enter" && current) { e.preventDefault(); openFile(current); return; }
    let to: string | null = null;
    if (mode === "grid") to = gridKey(order, focus, e.key, gridCols());
    else {
      const nav = navKey(order.map((k) => ({ key: k })), focus, e.key, 10);
      to = nav && "to" in nav ? nav.to : null;
    }
    if (!to) return;
    e.preventDefault();
    sel = e.shiftKey ? clickPick(sel, order, to, { shift: true }) : { picked: [to], anchor: to };
    clicked = to;
    queueMicrotask(() => itemEl(to!)?.scrollIntoView?.({ block: "nearest" }));
  }
  const ownKeyTarget = (e: KeyboardEvent) => !(e.target as HTMLElement | null)?.closest?.("input, textarea, select, [contenteditable]");
  const itemEl = (p: string) => [...(area?.querySelectorAll<HTMLElement>("[data-path]") ?? [])].find((x) => x.dataset.path === p);
  function gridCols(): number {
    const tiles = [...(area?.querySelectorAll<HTMLElement>(".card") ?? [])];
    const top = tiles[0]?.offsetTop;
    return Math.max(1, tiles.filter((x) => x.offsetTop === top).length);
  }

  // A box drawn on empty space picks what it touches (Ctrl: adds to it);
  // dragging a picked file takes the picked files out of the app (to
  // Explorer, a DAW...): a native drag, started while the button is down.
  let marquee = $state<{ x0: number; y0: number; x1: number; y1: number } | null>(null);
  let press: { x: number; y: number; path: string; ctrl: boolean; before: string[] } | null = null;
  let suppressClick = false;
  let draggingOut = $state(false);
  function onPointerDown(e: PointerEvent) {
    if (e.button !== 0 || renaming) return;
    const item = (e.target as HTMLElement).closest<HTMLElement>("[data-path]");
    if ((e.target as HTMLElement).closest("input, .head")) return;
    press = { x: e.clientX, y: e.clientY, path: item?.dataset.path ?? "", ctrl: e.ctrlKey || e.metaKey, before: [...sel.picked] };
    suppressClick = false;
  }
  function onPointerMove(e: PointerEvent) {
    if (!press || !(e.buttons & 1)) { press = null; marquee = null; return; }
    const far = Math.abs(e.clientX - press.x) + Math.abs(e.clientY - press.y) > 6;
    if (!press.path) {
      if (!marquee && !far) return;
      marquee = { x0: press.x, y0: press.y, x1: e.clientX, y1: e.clientY };
      const box = { left: Math.min(press.x, e.clientX), right: Math.max(press.x, e.clientX), top: Math.min(press.y, e.clientY), bottom: Math.max(press.y, e.clientY) };
      const items = [...(area?.querySelectorAll<HTMLElement>("[data-path]") ?? [])].map((el) => ({ key: el.dataset.path!, rect: el.getBoundingClientRect() }));
      const picked = marqueePick(box, items, press.ctrl ? press.before : []);
      sel = { picked, anchor: picked[0] ?? "" };
      return;
    }
    if (far) {
      const path = press.path;
      press = null;
      dragOut(sel.picked.includes(path) ? order.filter((p) => sel.picked.includes(p)) : [path]);
    }
  }
  function onPointerUp() {
    if (press && !press.path && !marquee && !press.ctrl) sel = { picked: [], anchor: "" }; // a click on empty space
    press = null;
    marquee = null;
  }
  async function dragOut(paths: string[], fromTree = false) {
    if (!fromTree && !paths.every((p) => sel.picked.includes(p))) sel = { picked: paths, anchor: paths[0] };
    const there = paths.filter((p) => files.find((f) => f.path === p)?.status !== "deleted" || fromTree);
    if (!there.length || draggingOut) return;
    draggingOut = true;
    suppressClick = true;
    try {
      await api.StartDrag(root, there);
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      draggingOut = false;
      setTimeout(() => (suppressClick = false), 0);
    }
  }
  // (a tree folder dragged out: the folder itself)
  let treePress: { x: number; y: number; path: string } | null = null;
  function onTreeMove(e: PointerEvent) {
    if (!treePress || !(e.buttons & 1)) { treePress = null; return; }
    if (Math.abs(e.clientX - treePress.x) + Math.abs(e.clientY - treePress.y) > 6) {
      const p = treePress.path;
      treePress = null;
      if (p) dragOut([p], true);
    }
  }

  // --- files dropped from Explorer: copied into the folder they land on ---
  $effect(() => {
    const off = Events.On("files-dropped", (ev: { data: { files: string[]; root: string; dir: string } }) => {
      const d = ev.data;
      if (d.root !== root || draggingOut || !d.files?.length) return;
      copyIn(d.dir ?? "", d.files);
    });
    return () => off();
  });
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
      sel = { picked: res.copied.filter((p) => dirOf(p) === into), anchor: res.copied[0] ?? "" };
    } catch (e) {
      toast(errorText(e), "error", 9000);
    }
  }

  // --- renaming (F2, or the menu) ---
  let renaming = $state<{ path: string; dir: boolean; value: string } | null>(null);
  function startRename(path: string, isDir: boolean) {
    menu = null;
    renaming = { path, dir: isDir, value: baseName(path) };
  }
  // The name typed, the part before the extension picked.
  function nameField(el: HTMLInputElement) {
    el.focus();
    const dot = renaming?.dir ? -1 : el.value.lastIndexOf(".");
    el.setSelectionRange(0, dot > 0 ? dot : el.value.length);
  }
  async function finishRename(commit: boolean) {
    const r = renaming;
    if (!r) return;
    renaming = null;
    if (!commit || r.value.trim() === baseName(r.path)) return;
    try {
      const to = await api.RenameFile(root, r.path, r.value);
      if (r.dir) {
        const moved = (p: string) => (p === r.path || p.startsWith(r.path + "/") ? to + p.slice(r.path.length) : p);
        dir = moved(dir);
        modes = Object.fromEntries(Object.entries(modes).map(([k, v]) => [moved(k), v]));
      }
      await loadFiles();
      if (!r.dir) { sel = { picked: [to], anchor: to }; clicked = to; }
    } catch (e) {
      toast(errorText(e), "error", 9000);
    }
  }
  function renameKey(e: KeyboardEvent) {
    e.stopPropagation();
    if (e.key === "Enter") { e.preventDefault(); finishRename(true); }
    else if (e.key === "Escape") { e.preventDefault(); finishRename(false); }
  }

  // --- the menu of a file or folder (right click) ---
  let menu = $state<{ path: string; x: number; y: number; dir: boolean; ignore: IgnoreOption[] } | null>(null);
  let ignoreOpen = $state(false);
  let menuW = $state(220), menuH = $state(200);
  let converting = $state("");
  function openMenu(e: MouseEvent, path: string, isDir: boolean) {
    e.preventDefault();
    if (!isDir && !sel.picked.includes(path)) sel = { picked: [path], anchor: path };
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

  const sym: Record<string, string> = { added: "A", modified: "M", deleted: "D", renamed: "R" };
  const statusName = (s: string) => ({ added: t("New"), modified: t("Changed"), deleted: t("Deleted"), renamed: t("Moved") } as Record<string, string>)[s] ?? "";
  let changedTotal = $derived(tree.changed);
  const emptyText = () => filter ? t("No files here match “{text}”.", { text: filter })
    : findFolder(tree, dir)?.folders.length ? t("No files right in this folder: its folders are on the left.")
    : t("No files in this folder. Drop files here to copy them in.");
  let thumbFailed = $state<Record<string, boolean>>({});
</script>

<svelte:window onclick={(e) => { if (menu && !(e.target as HTMLElement).closest(".ctx")) menu = null; }}
  onkeydown={(e) => { if (e.key === "Escape" && menu) menu = null; }} onpointerup={onPointerUp} />

<div class="files-tab">
  <!-- the folders -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
  <nav class="tree card-box" aria-label={t("Folders")} tabindex="-1" onkeydown={onTreeKey}
    onpointermove={onTreeMove} onpointerup={() => (treePress = null)}>
    <div class="tree-h">
      <span>{t("Folders")}</span>
      {#if changedTotal}<span class="faint">{tn(changedTotal, "{n} changed", "{n} changed")}</span>{/if}
    </div>
    <ul>
      {#each treeRows as { f, depth } (f.path)}
        <li style:padding-left="{depth * 16}px">
          {#if f.folders.length && f.path !== ""}
            <button class="chev ghost" aria-label={isOpen(f.path) ? t("Close folder") : t("Open folder")} onclick={() => (open[f.path] = !isOpen(f.path))}>
              <svg class:open={isOpen(f.path)} viewBox="0 0 10 10" aria-hidden="true"><path d="M3 1.5 L7 5 L3 8.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg>
            </button>
          {:else}<span class="chev"></span>{/if}
          {#if renaming && renaming.dir && renaming.path === f.path}
            <input class="rename" bind:value={renaming.value} use:nameField onkeydown={renameKey} onblur={() => finishRename(true)}
              aria-label={t("New name")} />
          {:else}
            <button class="folder" class:on={f.path === dir} title={f.path || st.name} data-file-drop-target data-root={root} data-dir={f.path}
              onclick={() => pickDir(f.path)} ondblclick={() => f.folders.length && (open[f.path] = !isOpen(f.path))}
              oncontextmenu={(e) => f.path && openMenu(e, f.path, true)}
              onpointerdown={(e) => { if (e.button === 0 && f.path) treePress = { x: e.clientX, y: e.clientY, path: f.path }; }}
              ondragstart={(e) => e.preventDefault()}>
              <FileIcon kind="folder" open={f.path === dir} />
              <span class="fname">{f.name || t("Project")}</span>
              {#if f.changed}<span class="pill" title={tn(f.changed, "{n} changed file inside", "{n} changed files inside")}>{f.changed}</span>{/if}
            </button>
          {/if}
        </li>
      {/each}
    </ul>
    <p class="note faint">{t("Numbers: changed files inside. The files of the folder you pick are listed in the middle.")}</p>
  </nav>

  <!-- the folder's files, and the file picked -->
  <section class="area-card card-box">
    <div class="files">
      <div class="toolbar">
        <input class="filter" type="search" bind:value={filter} placeholder={t("Filter in {folder}", { folder: dir ? baseName(dir) : st.name })}
          aria-label={t("Filter")} />
        <div class="modes" role="group" aria-label={t("Display")}>
          <button class:on={mode === "list"} aria-pressed={mode === "list"} onclick={() => (modes[dir] = "list")} title={t("List view")} aria-label={t("List view")}>
            <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 4h10M3 8h10M3 12h10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" fill="none" /></svg>
          </button>
          <button class:on={mode === "grid"} aria-pressed={mode === "grid"} onclick={() => (modes[dir] = "grid")} title={t("Grid view")} aria-label={t("Grid view")}>
            <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M2.5 2.5h4.5v4.5h-4.5zM9 2.5h4.5v4.5h-4.5zM2.5 9h4.5v4.5h-4.5zM9 9h4.5v4.5h-4.5z" stroke="currentColor" stroke-width="1.4" fill="none" /></svg>
          </button>
        </div>
      </div>
      <p class="crumbs">
        <button class="crumb" class:at={!dir} onclick={() => pickDir("")}>{st.name}</button>
        {#each crumbs as c (c.path)}<span class="sep">›</span><button class="crumb" class:at={c.path === dir} onclick={() => pickDir(c.path)}>{c.name}</button>{/each}
        <span class="faint"> · {tn(here.length, "{n} file", "{n} files")} · {formatBytes(hereSize)} · {t("on")}</span>
        <span class="branch" style:--c="var(--lane-{branchLane(st.branches, st.branch)})"><span class="dot"></span>{branchLabel(st.branches, st.branch)}</span>
      </p>

      <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_static_element_interactions -->
      <div class="area {mode}" role={mode === "list" ? "grid" : "listbox"} aria-label={t("Files")} aria-multiselectable="true" tabindex="-1"
        bind:this={area} onkeydown={onAreaKey} onpointerdown={onPointerDown} onpointermove={onPointerMove}
        data-file-drop-target data-root={root} data-dir={dir}>
        {#if mode === "list"}
          <div class="row head" role="row">
            <span></span>
            <button role="columnheader" onclick={() => sortBy("name")}>{t("Name")} · {t("Last version")}{#if sort.key === "name"} {sort.desc ? "↓" : "↑"}{/if}</button>
            <span></span>
            <button role="columnheader" class="num" onclick={() => sortBy("size")} aria-label={t("Size")}>{t("Size")} {sort.key === "size" ? (sort.desc ? "↓" : "↑") : "↕"}</button>
            <button role="columnheader" onclick={() => sortBy("change")}>{t("Last change")}{#if sort.key === "change"} {sort.desc ? "↓" : "↑"}{/if}</button>
          </div>
          {#each shown as f (f.path)}
            {@const v = last[f.path]}
            {@const tool = toolOf(f.path)}
            <div class="row item {f.status}" role="row" aria-selected={sel.picked.includes(f.path)} data-path={f.path} class:on={sel.picked.includes(f.path)}
              class:focus={f.path === focus} title={f.path} tabindex="-1" onclick={(e) => pick(e, f.path)} onkeydown={() => {}} ondblclick={() => openFile(f)}
              oncontextmenu={(e) => openMenu(e, f.path, false)} ondragstart={(e) => e.preventDefault()}>
              <span class="badge" role="gridcell" title={tool.name || undefined}>{tool.badge}</span>
              <span class="nm" role="gridcell">
                {#if renaming && !renaming.dir && renaming.path === f.path}
                  <input class="rename" bind:value={renaming.value} use:nameField onkeydown={renameKey} onblur={() => finishRename(true)}
                    onclick={(e) => e.stopPropagation()} aria-label={t("New name")} />
                {:else}
                  <span class="fname">{baseName(f.path)}</span>
                {/if}
                <span class="msg">{v ? v.message || t("(no description)") : isChanged(f) ? t("Not committed yet") : ""}</span>
              </span>
              <span role="gridcell">{#if sym[f.status]}<span class="st {f.status}" title={statusName(f.status)}>{sym[f.status]}</span>{:else}<span class="faint">—</span>{/if}</span>
              <span class="num faint" role="gridcell">{formatBytes(f.size)}</span>
              <span class="who" role="gridcell">
                {#if v}<Avatar name={v.author} seed={v.authorId} color={looks?.[v.authorId]?.color ?? ""} picture={looks?.[v.authorId]?.picture ?? ""} size={18} />
                  <span class="wtxt">{v.author} · {ago(v.time)}</span>{/if}
              </span>
            </div>
          {:else}
            <p class="muted empty">{emptyText()}</p>
          {/each}
        {:else}
          {#each shown as f (f.path)}
            {@const v = last[f.path]}
            {@const tool = toolOf(f.path)}
            <div class="card {f.status}" data-path={f.path} class:on={sel.picked.includes(f.path)} class:focus={f.path === focus} title={f.path}
              role="option" aria-selected={sel.picked.includes(f.path)} tabindex="-1"
              onclick={(e) => pick(e, f.path)} ondblclick={() => openFile(f)} oncontextmenu={(e) => openMenu(e, f.path, false)}
              ondragstart={(e) => e.preventDefault()} onkeydown={() => {}}>
              <span class="thumb">
                <span class="ph">{#if tool.name}<span class="big-badge">{tool.badge}</span>{:else}<FileIcon kind={f.kind} />{/if}</span>
                {#if f.preview && f.status !== "deleted" && !thumbFailed[f.path]}
                  <img src="{previewURL(root, f.path, '', 320)}&t={f.modified}" alt="" loading="lazy" draggable="false"
                    onerror={() => (thumbFailed[f.path] = true)} />
                {/if}
                {#if sym[f.status]}<span class="st corner {f.status}" title={statusName(f.status)}>{sym[f.status]}</span>{/if}
              </span>
              {#if renaming && !renaming.dir && renaming.path === f.path}
                <input class="rename" bind:value={renaming.value} use:nameField onkeydown={renameKey} onblur={() => finishRename(true)}
                  onclick={(e) => e.stopPropagation()} aria-label={t("New name")} />
              {:else}
                <span class="cname">{baseName(f.path)}</span>
              {/if}
              <span class="cmeta">
                {#if v}<Avatar name={v.author} seed={v.authorId} color={looks?.[v.authorId]?.color ?? ""} picture={looks?.[v.authorId]?.picture ?? ""} size={16} />{/if}
                <span class="faint">{formatBytes(f.size)}</span>
              </span>
            </div>
          {:else}
            <p class="muted empty">{emptyText()}</p>
          {/each}
        {/if}
        {#if marquee}
          {@const r = area?.getBoundingClientRect()}
          <div class="marquee" style:left="{Math.min(marquee.x0, marquee.x1) - (r?.left ?? 0) + (area?.scrollLeft ?? 0)}px"
            style:top="{Math.min(marquee.y0, marquee.y1) - (r?.top ?? 0) + (area?.scrollTop ?? 0)}px"
            style:width="{Math.abs(marquee.x1 - marquee.x0)}px" style:height="{Math.abs(marquee.y1 - marquee.y0)}px"></div>
        {/if}
      </div>
    </div>

    <aside class="panel">
      {#if current}
        <FilePanel {root} {st} file={current} stamp={loadedAt} {onrestore} />
      {:else}
        <p class="muted pick-note">{sel.picked.length > 1 ? tn(sel.picked.length, "{n} file picked", "{n} files picked") : t("Pick a file to see it and its history.")}</p>
      {/if}
    </aside>
  </section>
</div>

{#if menu}
  {@const m = menu}
  {@const f = m.dir ? undefined : files.find((x) => x.path === m.path)}
  <div class="ctx surface-menu" role="menu" use:portal bind:offsetWidth={menuW} bind:offsetHeight={menuH}
    style:left="{Math.max(8, Math.min(m.x, window.innerWidth - menuW - 8))}px" style:top="{Math.max(8, Math.min(m.y, window.innerHeight - menuH - 8))}px">
    {#if f && f.status !== "deleted"}
      {@const tool = toolOf(f.path)}
      <button class="item" onclick={() => openFile(f)}>{tool.name ? t("Open in {tool}", { tool: tool.name }) : t("Open")}</button>
    {/if}
    {#if m.dir || (f && f.status !== "deleted")}
      <button class="item" onclick={() => showInFolder(m.path)}>{t("Show in folder")}</button>
      <button class="item" onclick={() => startRename(m.path, m.dir)}>{t("Rename")}<span class="faint key">F2</span></button>
    {/if}
    {#if f?.kind === "audio" && f.status !== "deleted"}
      <button class="item" onclick={() => { converting = m.path; menu = null; }}>{t("Convert…")}</button>
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

{#if converting}
  <ConvertDialog {root} file={converting} onclose={() => (converting = "")}
    ondone={async (p) => { converting = ""; toast(t("Converted to {file}", { file: baseName(p) }), "ok"); await loadFiles(); pickDir(dirOf(p)); sel = { picked: [p], anchor: p }; }} />
{/if}

<style>
  .files-tab { display: grid; grid-template-columns: 228px minmax(0, 1fr); gap: var(--sp-16); height: 100%; min-height: 0; padding: var(--sp-16); }
  .card-box { min-height: 0; border-radius: var(--radius-card); background: var(--panel); box-shadow: var(--shadow-card); }
  .tree { display: flex; flex-direction: column; overflow: hidden; padding: var(--sp-14) var(--sp-8) var(--sp-12); }
  .tree:focus { outline: none; }
  .tree-h { display: flex; justify-content: space-between; align-items: baseline; padding: 0 var(--sp-10) var(--sp-10);
    font-size: var(--fs-xs); font-weight: var(--fw-semibold); color: var(--muted); text-transform: uppercase; letter-spacing: .08em; }
  .tree-h .faint { text-transform: none; letter-spacing: 0; font-weight: normal; }
  .tree ul { flex: 1; min-height: 0; overflow: auto; list-style: none; margin: 0; padding: 0; }
  .tree li { display: flex; align-items: center; }
  .chev { flex: none; width: 18px; height: 22px; padding: 0; border: none; background: transparent; display: inline-flex; align-items: center; justify-content: center; }
  .chev svg { width: 10px; height: 10px; color: var(--muted); transition: transform .12s; }
  .chev svg.open { transform: rotate(90deg); }
  .folder { flex: 1; min-width: 0; display: flex; align-items: center; gap: var(--sp-8); border: none; background: transparent; text-align: left;
    padding: var(--sp-6) var(--sp-8); border-radius: var(--radius); font-size: var(--fs-md); }
  .folder:hover:not(.on) { background: var(--hover-soft); }
  .folder.on { background: var(--accent-soft); font-weight: var(--fw-semibold); }
  .fname { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .pill { flex: none; min-width: 18px; padding: 0 var(--sp-6); border-radius: var(--radius-pill); background: var(--warn-soft); color: var(--warn);
    font-size: var(--fs-2xs); font-weight: var(--fw-bold); text-align: center; }
  .note { flex: none; margin: var(--sp-10) var(--sp-10) 0; font-size: var(--fs-xs); line-height: 1.5; }
  :global(.file-drop-target-active) { outline: 2px dashed var(--accent); outline-offset: -2px; background: var(--accent-soft) !important; }

  .area-card { position: relative; display: flex; overflow: hidden; }
  .files { flex: 1; min-width: 0; display: flex; flex-direction: column; padding: var(--sp-14) var(--sp-16) 0;
    margin-right: calc(clamp(280px, 38%, 400px) + var(--sp-16)); }
  .toolbar { display: flex; align-items: center; gap: var(--sp-10); }
  .filter { flex: 0 1 220px; min-width: 120px; padding: var(--sp-6) var(--sp-10); font-size: var(--fs-sm); border-radius: var(--radius); }
  .modes { display: flex; margin-left: auto; padding: 2px; border-radius: var(--radius-pill); background: var(--bg-sunken); }
  .modes button { display: inline-flex; padding: var(--sp-4) var(--sp-10); border: none; border-radius: var(--radius-pill); background: transparent; color: var(--muted); }
  .modes button.on { background: var(--text); color: var(--bg); }
  .modes svg { width: 14px; height: 14px; }
  .crumbs { display: flex; align-items: center; flex-wrap: wrap; gap: var(--sp-2) var(--sp-4); margin: var(--sp-10) 0 var(--sp-8); font-size: var(--fs-sm); color: var(--muted); }
  .crumb { border: none; background: transparent; padding: 0; color: var(--muted); font-size: var(--fs-sm); }
  .crumb.at { color: var(--text); font-weight: var(--fw-semibold); }
  .sep { color: var(--faint); }
  .branch { display: inline-flex; align-items: center; gap: var(--sp-4); color: var(--c); }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--c); }

  .area { position: relative; flex: 1; min-height: 0; overflow: auto; padding-bottom: var(--sp-16); user-select: none; }
  .area:focus { outline: none; }
  .row { display: grid; grid-template-columns: 30px minmax(120px, 1fr) 34px 76px minmax(110px, 150px); align-items: center; gap: var(--sp-10);
    padding: 0 var(--sp-8); border-radius: var(--radius); }
  .row.head { position: sticky; top: 0; z-index: 1; height: 30px; background: var(--panel); }
  .row.head button { border: none; background: transparent; padding: 0; text-align: left; font-size: var(--fs-2xs); color: var(--faint);
    text-transform: uppercase; letter-spacing: .06em; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .row.head button:hover { color: var(--text); background: transparent; }
  .row.item { height: 48px; border-bottom: var(--border-width) solid var(--line-soft); cursor: default; }
  .row.item:hover:not(.on) { background: var(--hover-soft); }
  .row.on, .card.on { background: var(--accent-soft); }
  .row.deleted .fname, .card.deleted .cname { text-decoration: line-through; color: var(--faint); }
  .badge, .big-badge { display: inline-flex; align-items: center; justify-content: center; border-radius: var(--radius-sm);
    background: var(--hover-strong); color: var(--muted); font-weight: var(--fw-bold); }
  .badge { width: 24px; height: 20px; font-size: var(--fs-2xs); }
  .big-badge { min-width: 44px; height: 36px; padding: 0 var(--sp-6); font-size: var(--fs-md); border-radius: var(--radius); }
  .nm { min-width: 0; display: flex; flex-direction: column; line-height: 1.3; }
  .nm .fname { font-weight: var(--fw-semibold); font-size: var(--fs-md); }
  .msg { font-size: var(--fs-xs); color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .num { text-align: right; font-size: var(--fs-sm); }
  .who { min-width: 0; display: flex; align-items: center; gap: var(--sp-6); font-size: var(--fs-sm); color: var(--muted); }
  .wtxt { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .st { display: inline-flex; font-size: var(--fs-2xs); font-weight: var(--fw-bold); padding: 1px var(--sp-6); border-radius: var(--radius-sm);
    background: var(--mod-soft); color: var(--mod); }
  .st.added { background: var(--add-soft); color: var(--add); }
  .st.deleted { background: var(--del-soft); color: var(--del); }
  .st.renamed { background: var(--warn-soft); color: var(--warn); }
  .rename { width: 100%; padding: var(--sp-2) var(--sp-6); font-size: var(--fs-md); }

  .area.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: var(--sp-12); align-content: start; padding-top: var(--sp-4); }
  .card { display: flex; flex-direction: column; gap: var(--sp-6); padding: var(--sp-8); border-radius: var(--radius-lg); background: var(--panel-2);
    border: 2px solid transparent; min-width: 0; cursor: default; }
  .card:hover:not(.on) { background: var(--panel-3); }
  .card.on { border-color: var(--accent); }
  .card:focus { outline: none; }
  .thumb { position: relative; aspect-ratio: 1 / 0.95; border-radius: var(--radius); overflow: hidden; background: var(--bg-sunken); }
  .ph { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; color: var(--faint); }
  .ph :global(svg) { width: 40px; height: 40px; }
  .thumb img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; }
  .corner { position: absolute; top: var(--sp-6); left: var(--sp-6); }
  .cname { font-size: var(--fs-sm); font-weight: var(--fw-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .cmeta { display: flex; align-items: center; gap: var(--sp-6); font-size: var(--fs-xs); }
  .marquee { position: absolute; z-index: 2; pointer-events: none; border: var(--border-width) solid var(--accent); background: var(--accent-soft); border-radius: var(--radius-xs); }
  .empty { padding: var(--sp-16) var(--sp-8); }

  /* the file, floating over the card's right side */
  .panel { position: absolute; top: var(--sp-16); right: var(--sp-16); bottom: var(--sp-16); width: clamp(280px, 38%, 400px); display: flex; flex-direction: column;
    overflow: hidden; border-radius: var(--radius-xl); background: var(--surface-float); backdrop-filter: var(--float-filter); box-shadow: var(--shadow-float); }
  .panel > :global(*) { flex: 1; min-height: 0; }
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
