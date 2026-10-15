<script lang="ts">
  import { t, tn } from "../i18n";
  import { errorText } from "../text";
  import { notify as toast } from "../notify";
  import { lockKindName } from "../lockKinds";
  import type { AgentDefaults, TeamSource, TeamWorkflowData } from "./source";

  // A team's Workflow tab (docs/design/team-settings.md): file locking (R3V
  // Cloud teams) and the defaults for agents, what projects do unless they
  // set their own; the projects that do, put back to the team's in one go.
  // Owners and admins change them on R3V Cloud; everyone sees.
  let { source, onlocks }: { source: TeamSource; onlocks?: (on: boolean) => void } = $props();

  let v = $state<TeamWorkflowData | null>(null);
  let error = $state("");
  let saving = $state(false);
  async function load() {
    try {
      v = await source.workflow();
      error = "";
    } catch (e) {
      error = errorText(e);
    }
  }
  $effect(() => { void source; load(); });

  async function change(f: () => Promise<unknown>) {
    saving = true;
    try {
      await f();
      await load();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      saving = false;
    }
  }
  const setLocks = (on: boolean, kinds: string[]) => change(async () => {
    await source.setLocks(on, kinds);
    onlocks?.(on);
  });
  const toggleKind = (id: string, on: boolean) => setLocks(true, on ? [...v!.kinds, id] : v!.kinds.filter((k) => k !== id));
  const setAgent = (k: keyof AgentDefaults, on: boolean) => change(() => source.setAgents({ ...v!.agents, [k]: on }));
  async function clear(pick: "locks" | "agents") {
    try {
      const n = await source.clearOverrides(pick);
      toast(tn(n, "{n} project follows the team again", "{n} projects follow the team again"), "ok");
      await load();
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
  let ownLocks = $derived(v?.overrides.filter((o) => o.locks) ?? []);
  let ownAgents = $derived(v?.overrides.filter((o) => o.agents) ?? []);
  let lockSummary = $derived(!v?.locksOn ? t("Off") : v.kinds.length
    ? t("On · auto-lock: {kinds}", { kinds: v.kinds.map(lockKindName).join(", ") }) : t("On · manual locks only"));
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
  <section>
    <h3>{t("File locking")}</h3>
    {#if !v.locks}
      <p class="hint">{t("File locking needs a team on R3V Cloud: it keeps who is changing which file.")}</p>
    {:else if v.canEdit}
      <label class="row"><input type="checkbox" checked={v.locksOn} disabled={saving} onchange={(e) => setLocks(e.currentTarget.checked, v!.kinds)} />
        {t("File locking")}</label>
      <p class="hint">{t("Off: no locks. On: anyone can lock a file or folder (right-click › Lock), and a change to a file someone else holds can't be shared until it's unlocked. With auto-lock, the kinds of file ticked below lock by themselves when someone changes them; none ticked: manual locks only.")}</p>
      {#if v.locksOn}
        <h4>{t("Auto-lock")}</h4>
        <div class="kinds">
          {#each v.all as k (k.id)}
            <label class="row kind"><input type="checkbox" checked={v.kinds.includes(k.id)} disabled={saving}
              onchange={(e) => toggleKind(k.id, e.currentTarget.checked)} />
              <span>{lockKindName(k.id)} <span class="faint mono">{k.patterns.join(" ")}</span></span></label>
          {/each}
        </div>
        <p class="hint">{t("This is what projects do unless they set their own, in their settings: on or off, other kinds, or paths of their own.")}</p>
      {:else}
        <p class="hint">{t("Turning it on: teammates on an older R3V must update before they can share.")}</p>
      {/if}
    {:else}
      <p class="small">{lockSummary}</p>
      <p class="hint">{t("Owners and admins change it.")}</p>
    {/if}
    {#if v.locks}{@render overrides(ownLocks, "locks")}{/if}
  </section>

  {#if v.agentsHere !== false}
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
  {/if}
{:else if error}
  <p class="error">{error}</p>
{/if}

<style>
  section { margin-bottom: var(--sp-18); }
  h3 { font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--muted); margin: 0 0 var(--sp-8); }
  h4 { font-size: var(--fs-md); margin: var(--sp-10) 0 var(--sp-6); }
  .hint { margin: var(--sp-2) 0 var(--sp-6); font-size: var(--fs-sm); color: var(--faint); }
  .row { display: flex; align-items: center; gap: var(--sp-8); color: var(--text); font-size: var(--fs-base); margin: 0 0 var(--sp-4); }
  .row input { width: auto; }
  .kinds { display: flex; flex-direction: column; gap: var(--sp-2); margin-bottom: var(--sp-8); }
  .kind { font-size: var(--fs-md); }
  .mono { font-family: var(--font-mono); font-size: var(--fs-xs); }
  .small { font-size: var(--fs-md); }
  .action { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--sp-12); padding: var(--sp-6) 0; }
  .action strong { font-weight: var(--fw-medium); }
  .own { margin: var(--sp-8) 0 var(--sp-14); font-size: var(--fs-md); }
  .own summary { cursor: pointer; color: var(--accent-text); }
  .own ul { margin: var(--sp-6) 0; padding-left: var(--sp-20); }
  .own button { font-size: var(--fs-sm); padding: var(--sp-4) var(--sp-10); }
  .error { color: var(--danger); }
</style>
