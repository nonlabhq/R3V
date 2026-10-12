<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, ago, errorText, toolName, type AgentWorkspace, type Branch, type TeamProject } from "./api";
  import { toast } from "./notify.svelte";
  import { branchLabel } from "./branches";
  import AgentBadge from "./AgentBadge.svelte";
  import LockBadge from "./LockBadge.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import Modal from "./Modal.svelte";
  import NewWorkspaceDialog from "./NewWorkspaceDialog.svelte";

  // The Agents view (Nightly; docs/design/agent-workspaces.md, "The
  // overview"): the project's workspaces on this computer, an agent's each,
  // with what each has done; merged into the branch it came from, opened,
  // removed. Read again when the project changes (version), and now and
  // then: agents work without telling the app.
  let { root, name, branch, branches, version, guide, guideOld = false, onmerge, onopen, onchanged }: {
    root: string; name: string;
    guide: boolean; // the project has the agents' guide (AGENTS.md)
    guideOld?: boolean; // R3V's part of it is an older R3V's
    branch: string; // the branch the project is on
    branches: Branch[];
    version: string; // changes when the project does (read again then)
    onmerge: (w: AgentWorkspace) => void; // the merge preview of its branch into its base
    onopen: (p: TeamProject) => void; // shown in the sidebar: go to it
    onchanged: () => void; // made or removed one (the tab's count)
  } = $props();

  let list = $state<AgentWorkspace[] | null>(null);
  let error = $state("");
  let creating = $state(false);
  let removing = $state<AgentWorkspace | null>(null);
  let refused = $state<{ w: AgentWorkspace; unmerged: number; changes: number; parked: number; base: string } | null>(null);
  let busy = $state(""); // the folder of a workspace being opened or removed
  let open = $state<Record<string, boolean>>({}); // locks listed, by folder

  // The agents' guide (AGENTS.md, imported by CLAUDE.md): offered until the
  // project has it; committed, every workspace then has it.
  let guiding = $state(false);
  async function addGuide() {
    guiding = true;
    try {
      const written = await api.AddAgentGuide(root);
      const files = written.join(", ");
      toast(guideOld ? t("Updated {files}: commit it, so every agent's workspace has it.", { files })
        : t("Added {files}: commit them, so every agent's workspace has them.", { files }), "ok", 9000);
      onchanged();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      guiding = false;
    }
  }

  let reading = false;
  async function load() {
    if (reading) return;
    reading = true;
    const r = root;
    try {
      const l = await api.AgentWorkspaces(r);
      if (r === root) { list = l ?? []; error = ""; }
    } catch (e) {
      if (r === root) error = errorText(e);
    } finally {
      reading = false;
    }
  }
  $effect(() => {
    root; version;
    load();
  });
  $effect(() => {
    const timer = setInterval(() => { if (!document.hidden) load(); }, 20000);
    return () => clearInterval(timer);
  });

  const bl = (key: string, label = "") => label || branchLabel(branches, key);
  // An agent heard of more than an hour ago is quiet; its locks may be stale.
  const QUIET = 3600 * 1000;
  const quiet = (w: AgentWorkspace) => w.stale || Date.now() - Date.parse(w.lastActivity) > QUIET;
  const fileName = (p: string) => { const s = p.replace(/\/$/, ""); return s.slice(s.lastIndexOf("/") + 1) + (p.endsWith("/") ? "/" : ""); };

  async function openWorkspace(w: AgentWorkspace) {
    busy = w.folder;
    try {
      const p = await api.OpenWorkspace(root, w.folder);
      onopen(p);
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      busy = "";
    }
  }

  async function remove(w: AgentWorkspace, force: boolean) {
    removing = null;
    refused = null;
    busy = w.folder;
    try {
      const r = await api.RemoveWorkspace(root, w.folder, force);
      if (r?.left) {
        refused = { w, ...r.left };
        return;
      }
      toast(w.missing ? t("“{branch}” is off the list", { branch: bl(w.branch, w.label) })
        : t("Removed the workspace “{branch}”. Its versions stay with the team.", { branch: bl(w.branch, w.label) }), "ok", 7000);
      onchanged();
    } catch (e) {
      toast(errorText(e), "error", 9000);
    } finally {
      busy = "";
      await load();
    }
  }

  function created(a: { folder: string; label: string; setupError: string }) {
    creating = false;
    toast(t("The workspace “{branch}” is ready in {folder}", { branch: a.label, folder: a.folder }), "ok", 8000);
    if (a.setupError) toast(t("Claude Code isn't set up in it: {error}", { error: a.setupError }), "warn", 12000);
    onchanged();
    load();
  }

  let command = $derived(`r3v workspace add "${root.replace(/[\\/][^\\/]*$/, "")}${root.includes("\\") ? "\\" : "/"}${name} (renderer)" --branch renderer --agent claude-code`);
  let copied = $state(false);
  async function copyCommand() {
    try {
      await navigator.clipboard.writeText(command);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch { /* not copied */ }
  }
</script>

<div class="agents">
  <div class="head">
    <h2>{t("Workspaces")}</h2>
    <span class="spacer"></span>
    <button class="primary" onclick={() => (creating = true)}>+ {t("New workspace")}</button>
  </div>

  {#if !guide}
    <div class="guide">
      <span>{t("Agents don't know yet that this project is versioned with R3V, or how to work beside each other. A short guide in the project tells them (AGENTS.md, which Claude Code, Codex and others read).")}</span>
      <button onclick={addGuide} disabled={guiding}>{t("Add the guide for agents")}</button>
    </div>
  {:else if guideOld}
    <div class="guide">
      <span>{t("R3V's guide for agents has changed since it was added to this project. Update R3V's part of AGENTS.md (the rest stays as it is), then commit it.")}</span>
      <button onclick={addGuide} disabled={guiding}>{t("Update the guide")}</button>
    </div>
  {/if}

  {#if error && !list}
    <p class="error">{error}</p>
  {:else if !list}
    <p class="muted">{t("Reading the workspaces…")}</p>
  {:else if !list.length}
    <div class="empty">
      <h3>{t("Agents working side by side")}</h3>
      <p>{t("A workspace is another copy of this project on this computer, on a branch of its own. Give each AI agent one: they work at the same time without getting in each other's way, and you merge what each has done when it's ready.")}</p>
      <p class="muted">{t("Files that can't be merged (textures, models, sounds) are locked by the agent changing them, so no two agents change the same one.")}</p>
      <p class="muted">{t("An agent can make its own from the command line:")}</p>
      <div class="cmd">
        <code>{command}</code>
        <button onclick={copyCommand}>{copied ? `✓ ${t("Copied")}` : t("Copy")}</button>
      </div>
    </div>
  {:else}
    <ul>
      {#each list as w (w.folder)}
        {@const label = bl(w.branch, w.label)}
        {@const base = bl(w.base, w.baseLabel)}
        <li class:missing={w.missing}>
          <div class="top">
            {#if w.tool}<AgentBadge tool={w.tool} title={t("Set up for {tool}", { tool: toolName(w.tool) })} />{/if}
            <strong class="branch">{label}</strong>
            <span class="faint">{t("from “{branch}”", { branch: base })}</span>
            <span class="spacer"></span>
            {#if !w.missing}
              <button class:primary={!!w.ahead} disabled={!w.ahead || !!busy} onclick={() => onmerge(w)}
                title={w.ahead ? t("See what comes in, then merge") : t("Nothing to merge yet")}>{t("Merge into {branch}", { branch: base })}</button>
              <button onclick={() => openWorkspace(w)} disabled={!!busy}
                title={t("Show it in the sidebar, to look at its versions and files")}>{t("Open")}</button>
            {/if}
            <button class="ghost danger" onclick={() => (removing = w)} disabled={!!busy}>{t("Remove…")}</button>
          </div>
          {#if w.missing}
            <p class="warn-text">{t("The folder is gone: remove the workspace from the list.")}</p>
          {:else}
            <button class="link folder mono" onclick={() => api.ShowFolder(w.folder)} title={t("Show in Explorer")}>{w.folder}</button>
          {/if}
          {#if !w.missing}
            <div class="facts">
              {#if w.ahead == null}
                <span class="faint">{t("What it has to merge isn't known: the team can't be reached")}</span>
              {:else if w.ahead > 0}
                <span class="fact to-merge">{tn(w.ahead, "{n} version to merge", "{n} versions to merge")}</span>
              {:else}
                <span class="fact">{t("Nothing to merge")}</span>
              {/if}
              {#if w.behind}<span class="fact">{tn(w.behind, "{n} version behind “{branch}”", "{n} versions behind “{branch}”", { branch: base })}</span>{/if}
              {#if w.changes}<span class="fact">{tn(w.changes, "{n} change not committed", "{n} changes not committed")}</span>{/if}
              {#if w.parked}<span class="fact">{tn(w.parked, "{n} set of parked changes", "{n} sets of parked changes")}</span>{/if}
              {#if w.locks.length}
                <button class="ghost fact locks" aria-expanded={!!open[w.folder]} onclick={() => (open[w.folder] = !open[w.folder])}>
                  <svg viewBox="0 0 16 16" aria-hidden="true"><rect x="3" y="7" width="10" height="7" rx="1.5" fill="currentColor" />
                    <path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2" fill="none" stroke="currentColor" stroke-width="1.6" /></svg>
                  {tn(w.locks.length, "Holds {n} lock", "Holds {n} locks")}</button>
              {/if}
              <span class="fact" class:quiet={quiet(w)}
                title={w.stale ? t("Its agent hasn't been heard from for a while: its locks can be taken over") : ""}>
                {quiet(w) ? t("Quiet since {when}", { when: ago(w.lastActivity) }) : t("Active {when}", { when: ago(w.lastActivity) })}</span>
              {#if w.owner}<span class="faint">{t("for {name}", { name: w.owner })}</span>{/if}
            </div>
            {#if open[w.folder] && w.locks.length}
              <ul class="lock-list">
                {#each w.locks as l (l.path)}
                  <li><LockBadge lock={l} branchName={(k) => bl(k)} /> <span class="mono" title={l.path}>{fileName(l.path)}</span> <span class="faint">{l.path}</span>
                    {#if l.since}<span class="faint">· {ago(l.since)}</span>{/if}</li>
                {/each}
              </ul>
            {/if}
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

{#if creating}
  <NewWorkspaceDialog {root} {name} {branch} {branches} oncreated={created} onclose={() => (creating = false)} />
{/if}

{#if removing}
  {@const w = removing}
  <ConfirmDialog title={t("Remove the workspace “{branch}”?", { branch: bl(w.branch, w.label) })} confirm={t("Remove")}
    text={w.missing ? t("It comes off the list. Its branch stays with the team.")
      : t("Its folder, {folder}, is deleted from this computer and its branch is archived. Its versions stay with the team.", { folder: w.folder })}
    onconfirm={() => remove(w, false)} onclose={() => (removing = null)} />
{/if}

{#if refused}
  {@const r = refused}
  <Modal title={t("“{branch}” has work that isn't merged", { branch: bl(r.w.branch, r.w.label) })} onclose={() => (refused = null)}>
    <p>{t("Nothing was removed. It still has:")}</p>
    <ul class="left">
      {#if r.unmerged}<li>{tn(r.unmerged, "{n} version not merged into “{branch}”", "{n} versions not merged into “{branch}”", { branch: r.base })}</li>{/if}
      {#if r.changes}<li>{tn(r.changes, "{n} change not committed", "{n} changes not committed")}</li>{/if}
      {#if r.parked}<li>{tn(r.parked, "{n} set of parked changes", "{n} sets of parked changes")}</li>{/if}
    </ul>
    <p class="muted">{t("Merge it first, or open it to commit what's there.")}</p>
    {#if r.changes || r.parked}
      <p class="warn-text">{t("Removing it anyway loses its changes not committed and parked, for good: they can't be brought back.")}</p>
    {/if}
    {#if r.unmerged}<p class="muted">{t("Versions not merged stay on its branch, with the team.")}</p>{/if}
    {#snippet footer()}
      <button onclick={() => (refused = null)}>{t("Keep the workspace")}</button>
      <button class="danger" onclick={() => remove(r.w, true)}>{t("Remove anyway")}</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .agents { max-width: 860px; }
  .head { display: flex; align-items: center; gap: var(--sp-8); margin-bottom: var(--sp-16); }
  h2 { margin: 0; font-size: var(--fs-xl); font-weight: var(--fw-semibold); }
  h3 { margin: 0 0 var(--sp-8); font-size: var(--fs-lg); font-weight: var(--fw-semibold); }
  .spacer { flex: 1; }
  .error { color: var(--danger); }
  .warn-text { color: var(--warn); margin: var(--sp-6) 0 0; }
  .guide { display: flex; align-items: center; gap: var(--sp-12); margin: 0 0 var(--sp-16); padding: var(--sp-10) var(--sp-12);
    border: var(--border-width) solid var(--line); border-radius: var(--radius); background: var(--panel); font-size: var(--fs-sm); color: var(--muted); }
  .guide span { flex: 1; line-height: 1.5; }
  .guide button { flex: none; }
  .empty { max-width: 620px; }
  .empty p { margin: 0 0 var(--sp-10); line-height: 1.5; }
  .cmd { display: flex; gap: var(--sp-8); align-items: center; }
  .cmd code { flex: 1; min-width: 0; padding: var(--sp-8) var(--sp-10); border-radius: var(--radius); background: var(--bg-sunken);
    font-family: var(--font-mono); font-size: var(--fs-sm); color: var(--muted); overflow-x: auto; white-space: nowrap; }
  .cmd button { flex: none; }
  ul { list-style: none; margin: 0; padding: 0; }
  li { padding: var(--sp-14) var(--sp-16); border: var(--border-width) solid var(--line); border-radius: var(--radius-lg);
    background: var(--panel-2); margin-bottom: var(--sp-10); }
  li.missing { border-color: var(--warn-line); }
  .top { display: flex; align-items: center; gap: var(--sp-8); min-width: 0; }
  .branch { font-size: var(--fs-lg); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .top .faint { font-size: var(--fs-sm); white-space: nowrap; }
  .top button { flex: none; }
  .folder { display: block; margin: var(--sp-6) 0 0; font-size: var(--fs-sm); max-width: 100%; overflow: hidden; text-overflow: ellipsis;
    white-space: nowrap; border: none; background: none; padding: 0; color: var(--muted); text-align: left; cursor: pointer; }
  .folder:hover:not(:disabled) { color: var(--text); background: none; text-decoration: underline; }
  .facts { display: flex; flex-wrap: wrap; align-items: center; gap: var(--sp-6) var(--sp-12); margin-top: var(--sp-10); font-size: var(--fs-sm); }
  .fact { color: var(--muted); }
  .fact.to-merge { color: var(--accent-text); font-weight: var(--fw-semibold); }
  .fact.quiet { color: var(--faint); }
  button.locks { display: inline-flex; align-items: center; gap: var(--sp-4); padding: 0 var(--sp-4); font-size: var(--fs-sm); }
  button.locks svg { width: 12px; height: 12px; }
  .lock-list { margin-top: var(--sp-8); }
  .lock-list li { border: none; background: none; padding: var(--sp-2) 0; margin: 0; font-size: var(--fs-sm); }
  .lock-list .mono { font-size: var(--fs-sm); }
  ul.left { list-style: disc; padding-left: var(--sp-20); margin: 0 0 var(--sp-10); }
  ul.left li { border: none; background: none; padding: var(--sp-2) 0; margin: 0; }
</style>
