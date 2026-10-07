<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, errorText, type ProjectFile, type Version } from "./api";
  import FileIcon from "./FileIcon.svelte";
  import Splitter from "./Splitter.svelte";
  import { splitPx } from "./splits.svelte";
  import { viewerFor, type Side } from "./viewers";
  import type { Snippet } from "svelte";

  // A version in the Overview: what it is (header, with what can be done
  // with it), the files it changed, and the picked one against the version
  // before it, shown by its kind's viewer.
  let { root, v, branch, actions }: {
    root: string; v: Version;
    branch: string;   // the branch it is on, when known
    actions?: Snippet; // buttons for this version (Merge, Go to, …)
  } = $props();

  let files = $state<ProjectFile[] | null>(null);
  let error = $state("");
  let picked = $state("");
  // By value: the project read again (on focus, say) brings the same
  // version as a new object, and the props (getters) follow the app's lists
  // read again too; neither must read the files again (the list and the
  // diff would blink, the picked file go back to the first).
  let vid = $derived(v.id);
  let at = $derived(root);
  let parent = $derived(v.parents[0] ?? "");
  $effect(() => {
    const id = vid;
    files = null;
    error = "";
    picked = "";
    api.VersionFiles(at, id).then((f) => {
      if (id !== vid) return;
      files = f ?? [];
      picked = files[0]?.path ?? "";
    }).catch((e) => {
      if (id === vid) error = errorText(e); // (not one for a version picked before)
    });
  });
  let current = $derived(files?.find((f) => f.path === picked));
  // The list's width: the same split as your changes' list.
  let bodyWidth = $state(0);
  let listWidth = $derived(splitPx("list", 0.34, bodyWidth, 240, 300));
  const name = (p: string) => p.slice(p.lastIndexOf("/") + 1);
  const dir = (p: string) => p.slice(0, p.lastIndexOf("/") + 1);
  const statusName = (s: string) => ({ added: t("New"), modified: t("Changed"), deleted: t("Deleted"),
    renamed: t("Moved"), untracked: t("No longer tracked") } as Record<string, string>)[s] ?? s;
  const sym: Record<string, string> = { added: "A", modified: "M", deleted: "D", renamed: "R", untracked: "U" };

  // This version (a) against the one before it (b).
  function sides(f: ProjectFile): { a: Side | null; b: Side | null } {
    return {
      a: f.status === "deleted" ? null : { path: f.path, version: vid, label: t("In this version") },
      b: f.status === "added" || !parent ? null : { path: f.status === "renamed" && f.from ? f.from : f.path, version: parent, label: t("Before") },
    };
  }
</script>

<div class="version">
  <header>
    <div class="title">
      <h2>{v.message || t("(no description)")}</h2>
      <div class="meta">
        {v.author} · {new Date(v.time).toLocaleString()}{#if branch} · {branch}{/if} · <span class="mono">{v.short}</span>
        {#if v.parents.length > 1} · {t("merge")}{/if}
      </div>
    </div>
    {#if actions}<div class="acts">{@render actions()}</div>{/if}
  </header>
  <div class="body" bind:clientWidth={bodyWidth} style:grid-template-columns="{listWidth}px 1fr">
    {#if bodyWidth}<Splitter key="list" def={0.34} width={bodyWidth} minLeft={240} minRight={300} />{/if}
    <aside>
      <div class="list-h">{files ? tn(files.length, "{count} file changed", "{count} files changed", { count: files.length }) : t("Changes")}</div>
      {#if error}
        <p class="error">{error}</p>
      {:else if files === null}
        <p class="muted pad">{t("Reading…")}</p>
      {:else if files.length === 0}
        <p class="muted pad">{t("No file changes.")}</p>
      {:else}
        <ul>
          {#each files as f (f.path)}
            <li>
              <button class:on={f.path === picked} onclick={() => (picked = f.path)} title={f.path}>
                <FileIcon kind={f.kind} />
                <span class="names"><span class="fname">{name(f.path)}</span>{#if dir(f.path)}<span class="fdir">{dir(f.path)}</span>{/if}</span>
                <span class="st {f.status}" title={statusName(f.status)}>{sym[f.status] ?? "?"}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </aside>
    <section class="diff">
      {#if current}
        {@const View = viewerFor(current).component}
        {@const s = sides(current)}
        <div class="diff-h">
          <div class="dname"><FileIcon kind={current.kind} /> {name(current.path)}</div>
          <div class="faint small">{current.status === "renamed" ? t("Moved from {path}", { path: current.from }) : statusName(current.status)}</div>
        </div>
        <View root={at} file={current} a={s.a} b={s.b} compare={true} stamp={0} />
      {:else if files && files.length}
        <p class="muted">{t("Pick a file on the left to see what changed.")}</p>
      {/if}
    </section>
  </div>
</div>

<style>
  .version { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  header { display: flex; align-items: flex-start; gap: var(--sp-12); padding: var(--sp-14) var(--sp-16);
    border-bottom: var(--border-width) solid var(--line); }
  .title { flex: 1; min-width: 0; }
  h2 { margin: 0; font-size: var(--fs-lg); font-weight: var(--fw-semibold); user-select: text; }
  .meta { font-size: var(--fs-sm); color: var(--faint); margin-top: var(--sp-2); }
  .acts { display: flex; gap: var(--sp-6); flex-wrap: wrap; justify-content: flex-end; }
  .acts :global(button) { padding: var(--sp-4) var(--sp-10); font-size: var(--fs-md); }
  .body { position: relative; flex: 1; min-height: 0; display: grid; grid-template-columns: 270px 1fr; }
  aside { border-right: var(--border-width) solid var(--line); overflow: auto; padding: var(--sp-8); }
  .list-h { font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); padding: var(--sp-4) var(--sp-6) var(--sp-8); }
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
  .st { flex: none; width: 16px; height: 16px; border-radius: var(--radius-sm); display: inline-flex; align-items: center;
    justify-content: center; font-size: var(--fs-2xs); font-weight: var(--fw-bold); background: var(--mod-soft); color: var(--mod); }
  .st.added { background: var(--add-soft); color: var(--add); }
  .st.deleted { background: var(--del-soft); color: var(--del); }
  .st.renamed { background: var(--warn-soft); color: var(--warn); }
  .diff { overflow: auto; padding: var(--sp-14) var(--sp-16); min-width: 0; }
  .diff-h { margin-bottom: var(--sp-10); }
  .dname { display: flex; align-items: center; gap: var(--sp-6); font-weight: var(--fw-semibold); }
  .small { font-size: var(--fs-sm); }
  .error { color: var(--danger); padding: 0 var(--sp-6); }
</style>
