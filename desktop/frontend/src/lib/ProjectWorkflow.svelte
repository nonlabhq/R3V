<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText } from "./api";
  import { toast } from "./notify.svelte";
  import { lockKindName } from "./lockKinds";
  import WorkflowChoice from "$shared/WorkflowChoice.svelte";
  import type { ProjectWorkflowView } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";
  import type { ProjectLocks, Workflow } from "../../bindings/github.com/nonlabhq/r3v/internal/remote/models";

  // A project's Workflow settings (docs/design/team-settings.md): each as
  // the team's unless the project sets its own (on an R3V Cloud team, its
  // admins change them); and what this computer keeps out for itself.
  let { root }: { root: string } = $props();

  let v = $state<ProjectWorkflowView | null>(null);
  let error = $state("");
  let saving = $state(false);
  async function load() {
    try {
      v = await api.ProjectWorkflow(root);
      error = "";
    } catch (e) {
      error = errorText(e);
    }
  }
  $effect(() => { void root; load(); });

  const own = (b: boolean | null | undefined) => (b === undefined ? null : b);
  function current(): Workflow {
    return { locks: v?.ownLocks ?? null, steps: own(v?.steps), prompts: own(v?.prompts), agentLocks: own(v?.agentLocks) } as Workflow;
  }
  async function save(w: Workflow) {
    saving = true;
    try {
      await api.SetProjectWorkflow(root, w);
      await load();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      saving = false;
    }
  }
  const set = (k: "steps" | "prompts" | "agentLocks", b: boolean | null) => save({ ...current(), [k]: b } as Workflow);

  // File locking: on or off, and (its own) the kinds and paths that lock by themselves.
  let locksOn = $derived(v ? (v.ownLocks?.on ?? v.teamLocksOn) : false);
  function setLocks(l: Partial<ProjectLocks> | null) {
    if (!l) return save({ ...current(), locks: null } as Workflow);
    const next = { ...(v?.ownLocks ?? {}), ...l } as ProjectLocks;
    const empty = next.on == null && next.kinds == null && !next.autoOff && !next.add?.length && !next.remove?.length;
    save({ ...current(), locks: empty ? null : next } as Workflow);
  }
  let kinds = $derived(v?.ownLocks?.kinds ?? v?.teamKinds ?? []);
  const toggleKind = (id: string, on: boolean) => setLocks({ kinds: on ? [...kinds, id] : kinds.filter((k) => k !== id) });
  const lines = (s: string) => s.split("\n").map((x) => x.trim()).filter(Boolean);

  async function moveRulesLocks() {
    try {
      await api.MoveFileLocksToSettings(root);
      toast(t("Moved to the project's settings; .r3v.yaml changed: commit it."), "ok", 7000);
      await load();
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
  async function setMine(off: boolean, noPrompts: boolean) {
    try {
      await api.SetStepSettings(root, { off, no_prompts: noPrompts });
      await load();
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
</script>

{#if v}
  {@const ro = !v.canEdit || saving}
  <section>
    <h3>{t("Workflow")}</h3>
    {#if v.team}
      <p class="hint">{t("Each follows the team's settings unless the project sets its own.")}
        {#if !v.canEdit}{t("Owners and admins change these.")}{/if}</p>
    {/if}
    {#if v.rulesLocks}
      <div class="note">
        <p>{t("This project's .r3v.yaml still has file_locks: R3V no longer reads them there; file locking is set here now.")}</p>
        {#if v.canEdit}<button onclick={moveRulesLocks}>{v.locks ? t("Move them here") : t("Take them out")}</button>{/if}
      </div>
    {/if}

    {#if v.locks}
      <WorkflowChoice label={t("File locking")} hint={t("Files can be locked for editing; a change to a file someone else holds can't be shared until it's unlocked.")}
        team={v.teamLocksOn} own={v.ownLocks?.on ?? null} disabled={ro}
        onchange={(b) => setLocks(b === null ? { on: undefined } : { on: b })} />
      {#if locksOn}
        <div class="sub">
          <label class="row"><input type="checkbox" checked={!!v.ownLocks?.autoOff} disabled={ro}
            onchange={(e) => setLocks({ autoOff: e.currentTarget.checked })} /> {t("Manual locks only (no auto-lock)")}</label>
          {#if !v.ownLocks?.autoOff}
            <p class="hint">{v.ownLocks?.kinds ? t("Auto-lock, for this project:") : t("Auto-lock, as the team's:")}</p>
            <div class="kinds">
              {#each v.all as k (k.id)}
                <label class="row kind"><input type="checkbox" checked={kinds.includes(k.id)} disabled={ro}
                  onchange={(e) => toggleKind(k.id, e.currentTarget.checked)} /> {lockKindName(k.id)}</label>
              {/each}
            </div>
            {#if v.ownLocks?.kinds && !ro}<button class="ghost small" onclick={() => setLocks({ kinds: undefined })}>{t("Use the team's kinds")}</button>{/if}
            <div class="paths">
              <label>{t("Also auto-lock (one path per line: Content/Maps/, *.umap)")}
                <textarea rows="2" disabled={ro} value={(v.ownLocks?.add ?? []).join("\n")}
                  onchange={(e) => setLocks({ add: lines(e.currentTarget.value) })}></textarea></label>
              <label>{t("Never auto-lock (one path per line)")}
                <textarea rows="2" disabled={ro} value={(v.ownLocks?.remove ?? []).join("\n")}
                  onchange={(e) => setLocks({ remove: lines(e.currentTarget.value) })}></textarea></label>
            </div>
          {/if}
        </div>
      {/if}
    {/if}

    <WorkflowChoice label={t("Record agents' steps")} hint={t("After each of an agent's tool calls that changes files, R3V keeps the files as they are, to go back to or compare.")}
      team={v.teamAgents.steps} own={own(v.steps)} disabled={ro} onchange={(b) => set("steps", b)} />
    <WorkflowChoice label={t("Keep the prompts")} hint={t("Each step shows the prompt that asked for it. Keys and passwords in prompts are taken out first.")}
      team={v.teamAgents.prompts} own={own(v.prompts)} disabled={ro} onchange={(b) => set("prompts", b)} />
    <WorkflowChoice label={t("Agents take turns on files that can't be merged")} hint={t("Between this project's workspaces on a computer: one changes such a file at a time.")}
      team={v.teamAgents.agentLocks} own={own(v.agentLocks)} disabled={ro} onchange={(b) => set("agentLocks", b)} />

    <h4>{t("On this computer")}</h4>
    <label class="row"><input type="checkbox" checked={!!v.mine.off} onchange={(e) => setMine(e.currentTarget.checked, !!v!.mine.no_prompts)} />
      {t("Make no steps here")}</label>
    <label class="row"><input type="checkbox" checked={!!v.mine.no_prompts} onchange={(e) => setMine(!!v!.mine.off, e.currentTarget.checked)} />
      {t("Don't keep my prompts")}</label>
    <p class="hint">{t("Only ever keeps more out than the settings above.")}</p>
  </section>
{:else if error}
  <p class="error">{error}</p>
{/if}

<style>
  section { margin-bottom: var(--sp-18); }
  h3 { font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--muted); margin: 0 0 var(--sp-8); }
  h4 { font-size: var(--fs-md); margin: var(--sp-14) 0 var(--sp-6); }
  .hint { margin: 0 0 var(--sp-6); font-size: var(--fs-sm); color: var(--faint); }
  .note { display: flex; align-items: center; gap: var(--sp-10); padding: var(--sp-8) var(--sp-10); margin-bottom: var(--sp-8);
    border-radius: var(--radius); background: var(--warn-bg); font-size: var(--fs-sm); }
  .note p { margin: 0; flex: 1; }
  .sub { margin: 0 0 var(--sp-8) var(--sp-16); padding-left: var(--sp-10); border-left: var(--border-width) solid var(--line); }
  .row { display: flex; align-items: center; gap: var(--sp-8); margin: 0 0 var(--sp-4); color: var(--text); font-size: var(--fs-md); }
  .row input { width: auto; }
  .kinds { display: flex; flex-direction: column; gap: var(--sp-2); margin-bottom: var(--sp-6); }
  .paths { display: flex; flex-direction: column; gap: var(--sp-6); margin-top: var(--sp-6); }
  .paths label { margin: 0; font-size: var(--fs-sm); }
  .paths textarea { width: 100%; font-family: var(--font-mono); font-size: var(--fs-sm); }
  .small { font-size: var(--fs-xs); padding: 0 var(--sp-4); }
  .error { color: var(--danger); }
</style>
