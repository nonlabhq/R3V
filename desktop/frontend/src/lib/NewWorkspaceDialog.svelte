<script lang="ts">
  import { untrack } from "svelte";
  import { t } from "./i18n.svelte";
  import { api, errorText, formatBytes, type Branch } from "./api";
  import { branchLabel } from "$shared/branches";
  import Modal from "./Modal.svelte";

  // A new workspace, for you or an AI agent: its branch's name, its folder
  // (beside the project's, named after it and the branch, until one is
  // typed or chosen), the branch it starts from, whether it is an agent's
  // (its branch marked so, Claude Code set up in it), and the room it takes.
  let { root, name, branch, branches, oncreated, onclose }: {
    root: string; name: string; // the project's folder and name
    branch: string; // the branch the project is on (where the workspace starts by default)
    branches: Branch[];
    oncreated: (added: { folder: string; label: string; setupError: string }) => void;
    onclose: () => void;
  } = $props();

  let label = $state("");
  let from = $state(untrack(() => branch)); // (where the project is, at first)
  let claude = $state(false); // an agent's (Claude Code set up in it)
  let folder = $state("");
  let folderTyped = $state(false);
  let busy = $state(false);
  let error = $state("");

  const sep = $derived(root.includes("\\") ? "\\" : "/");
  const parentOf = (p: string) => p.replace(/[\\/]+$/, "").replace(/[\\/][^\\/]*$/, "");
  // A folder name from the branch's: what Windows doesn't take in one, left out.
  const safe = (s: string) => s.replace(/[<>:"/\\|?*]+/g, "-").trim().replace(/[. ]+$/, "");
  let parent = $state(untrack(() => parentOf(root)));
  let suggested = $derived(`${parent}${sep}${safe(name)} (${safe(label) || "…"})`);
  $effect(() => { if (!folderTyped) folder = suggested; });

  // The room it takes (its drive may clone: none until files change).
  let room = $state<{ files: number; copies: number; clones: boolean } | null>(null);
  $effect(() => {
    const f = folder;
    const id = setTimeout(() => api.WorkspaceRoom(root, f).then((r) => { if (f === folder) room = r; }).catch(() => {}), 250);
    return () => clearTimeout(id);
  });

  async function choose() {
    const where = await api.ChooseFolder(t("Where should the workspace's folder go?")).catch(() => "");
    if (!where) return;
    parent = where.replace(/[\\/]+$/, "");
    folderTyped = false;
  }

  async function create() {
    if (!label.trim() || !folder.trim() || busy) return;
    busy = true;
    error = "";
    try {
      const added = await api.AddWorkspace(root, folder.trim(), label.trim(), from === branch ? "" : from,
        claude ? "claude-code" : "");
      if (added) oncreated({ folder: added.folder, label: added.label || label.trim(), setupError: added.setupError });
    } catch (e) {
      error = errorText(e);
    } finally {
      busy = false;
    }
  }
</script>

<Modal title={t("New workspace")} onclose={() => { if (!busy) onclose(); }} backdropCloses={false} escCloses={!busy}>
  <p class="muted">{t("Another copy of the project on this computer, on a branch of its own: to try something beside your main copy, or for an AI agent to work in. You merge its work when it's ready.")}</p>
  <label for="ws-branch">{t("Branch name")}</label>
  <!-- svelte-ignore a11y_autofocus -->
  <input id="ws-branch" bind:value={label} placeholder={t("renderer")} autofocus disabled={busy}
    onkeydown={(e) => { if (e.key === "Enter") create(); }} />
  <label for="ws-folder">{t("Folder")}</label>
  <div class="row">
    <input id="ws-folder" class="mono" bind:value={() => folder, (v) => { folder = v; folderTyped = true; }} disabled={busy} />
    <button onclick={choose} disabled={busy}>{t("Choose…")}</button>
  </div>
  {#if room}
    <p class="hint room faint">
      {room.copies ? t("Takes about {size}, {copies} of it build folders.", { size: formatBytes(room.files + room.copies), copies: formatBytes(room.copies) })
        : t("Takes about {size}.", { size: formatBytes(room.files) })}
      {room.clones ? t("This drive clones files: no room is taken until they change.") : ""}
    </p>
  {/if}
  <label for="ws-from">{t("Start from")}</label>
  <select id="ws-from" bind:value={from} disabled={busy}>
    {#each branches.filter((b) => b.kind !== "agent" || b.name === branch) as b (b.name)}
      <option value={b.name}>{branchLabel(branches, b.name)}{b.name === branch ? ` (${t("you are here")})` : ""}</option>
    {:else}
      <option value={branch}>{branchLabel(branches, branch)}</option>
    {/each}
  </select>
  <label class="check"><input type="checkbox" bind:checked={claude} disabled={busy} />{t("For an AI agent (Claude Code)")}</label>
  {#if claude}
    <p class="hint faint">{t("Its branch is marked an agent's. Claude Code then locks the files that can't be merged before changing them, and its versions say it made them.")}</p>
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#snippet footer()}
    <button onclick={onclose} disabled={busy}>{t("Cancel")}</button>
    <button class="primary" onclick={create} disabled={!label.trim() || !folder.trim() || busy}>
      {busy ? t("Making the workspace…") : t("Create")}</button>
  {/snippet}
</Modal>

<style>
  label { display: block; margin: var(--sp-12) 0 var(--sp-4); }
  select { width: 100%; } /* (its look: style.css) */
  label.check input { width: auto; flex: none; margin: 0; }
  .row { display: flex; gap: var(--sp-8); }
  .row input { flex: 1; min-width: 0; font-size: var(--fs-sm); }
  .row button { flex: none; }
  label.check { display: flex; align-items: center; gap: var(--sp-8); margin-top: var(--sp-16); color: var(--text); font-size: var(--fs-base); }
  .hint { margin: var(--sp-4) 0 0 var(--sp-24); font-size: var(--fs-sm); }
  .hint.room { margin-left: 0; }
  .error { color: var(--danger); }
</style>
