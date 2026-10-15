<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, errorText, type TeamSummary } from "./api";
  import { toast } from "./notify.svelte";
  import type { TeamWorkflowView } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";

  // The team's Workflow tab, beside its file locking (TeamLocks): the
  // defaults for agents, and the projects that set some of their own, each
  // put back to the team's in one go (docs/design/team-settings.md).
  let { team, stamp = 0 }: { team: TeamSummary; stamp?: number } = $props();

  let v = $state<TeamWorkflowView | null>(null);
  let error = $state("");
  let saving = $state(false);
  async function load() {
    try {
      v = await api.TeamWorkflow(team.id);
      error = "";
    } catch (e) {
      error = errorText(e);
    }
  }
  $effect(() => { void team.id; void stamp; load(); });

  async function setAgent(k: "steps" | "prompts" | "agentLocks", on: boolean) {
    if (!v) return;
    saving = true;
    try {
      await api.SetTeamAgents(team.id, { ...v.agents, [k]: on });
      await load();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      saving = false;
    }
  }
  async function clear(pick: "locks" | "agents") {
    try {
      const n = await api.ClearProjectWorkflow(team.id, pick);
      toast(tn(n, "{n} project follows the team again", "{n} projects follow the team again"), "ok");
      await load();
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
  let ownLocks = $derived(v?.overrides.filter((o) => o.locks) ?? []);
  let ownAgents = $derived(v?.overrides.filter((o) => o.agents) ?? []);
</script>

{#snippet overrides(list: { projectId: string; name: string }[], pick: "locks" | "agents")}
  {#if list.length}
    <details class="own">
      <summary>{tn(list.length, "{n} project sets its own", "{n} projects set their own")}</summary>
      <ul>{#each list as o (o.projectId)}<li>{o.name}</li>{/each}</ul>
      {#if v?.canEdit}<button onclick={() => clear(pick)}>{t("Apply the team's to every project")}</button>{/if}
    </details>
  {/if}
{/snippet}

{#if v}
  {#if v.locks}{@render overrides(ownLocks, "locks")}{/if}
  <section>
    <h3>{t("Agents")}</h3>
    <p class="hint">{t("What projects do unless they set their own (in their settings).")}
      {#if !v.canEdit}{t("Owners and admins change these.")}{/if}</p>
    {#each [["steps", t("Record agents' steps"), t("After each of an agent's tool calls that changes files, R3V keeps the files as they are, to go back to or compare.")],
      ["prompts", t("Keep the prompts"), t("Each step shows the prompt that asked for it. Keys and passwords in prompts are taken out first.")],
      ["agentLocks", t("Agents take turns on files that can't be merged"), t("Between a project's workspaces on a computer: one changes such a file at a time.")]] as const as [k, label, hint] (k)}
      <div class="action">
        <div><strong>{label}</strong><p class="hint">{hint}</p></div>
        <input type="checkbox" class="switch" role="switch" aria-label={label} checked={v.agents[k]} disabled={!v.canEdit || saving}
          onchange={(e) => setAgent(k, e.currentTarget.checked)} />
      </div>
    {/each}
    <p class="hint">{t("Changing these: teammates on an older R3V must update before they can share.")}</p>
    {@render overrides(ownAgents, "agents")}
  </section>
{:else if error}
  <p class="error">{error}</p>
{/if}

<style>
  section { margin-bottom: var(--sp-18); }
  h3 { font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--muted); margin: 0 0 var(--sp-8); }
  .hint { margin: var(--sp-2) 0 var(--sp-6); font-size: var(--fs-sm); color: var(--faint); }
  .action { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--sp-12); padding: var(--sp-6) 0; }
  .action strong { font-weight: var(--fw-medium); }
  .own { margin: 0 0 var(--sp-14); font-size: var(--fs-md); }
  .own summary { cursor: pointer; color: var(--accent-text); }
  .own ul { margin: var(--sp-6) 0; padding-left: var(--sp-20); }
  .own button { font-size: var(--fs-sm); padding: var(--sp-4) var(--sp-10); }
  .error { color: var(--danger); }
</style>
