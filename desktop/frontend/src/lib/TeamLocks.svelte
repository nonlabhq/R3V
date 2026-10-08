<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type TeamSummary } from "./api";
  import { toast } from "./notify.svelte";
  import { lockKindName } from "./lockKinds";
  import type { TeamLocks } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";

  // A hosted team's file locking (docs/design/locks.md): off, manual locks
  // only, or manual locks and auto-lock for the kinds of file ticked. Admins
  // change it; everyone sees it.
  let { team }: { team: TeamSummary } = $props();

  let s = $state<TeamLocks | null>(null);
  let error = $state("");
  let saving = $state(false);
  $effect(() => {
    const id = team.id;
    api.TeamLocks(id).then((v) => (s = v)).catch((e) => (error = errorText(e)));
  });

  async function save(on: boolean, kinds: string[]) {
    if (!s) return;
    saving = true;
    error = "";
    try {
      await api.SetTeamLocks(team.id, on, kinds);
      s = { ...s, on, kinds };
      if (on) toast(t("File locking is on for {team}", { team: team.name }), "ok");
    } catch (e) {
      error = errorText(e);
    } finally {
      saving = false;
    }
  }
  const toggleKind = (id: string, on: boolean) => save(true, on ? [...s!.kinds, id] : s!.kinds.filter((k) => k !== id));
  let summary = $derived(!s?.on ? t("Off") : s.kinds.length
    ? t("On · auto-lock: {kinds}", { kinds: s.kinds.map(lockKindName).join(", ") }) : t("On · manual locks only"));
</script>

{#if s?.available}
  <section>
    <h3>{t("File locking")}</h3>
    {#if s.admin}
      <label class="row"><input type="checkbox" checked={s.on} disabled={saving} onchange={(e) => save(e.currentTarget.checked, s!.kinds)} />
        {t("File locking")}</label>
      <p class="faint small">{t("Off: no locks. On: anyone can lock a file or folder (right-click › Lock), and a change to a file someone else holds can't be shared until it's unlocked. With auto-lock, the kinds of file ticked below lock by themselves when someone changes them; none ticked: manual locks only.")}</p>
      {#if s.on}
        <h4>{t("Auto-lock")}</h4>
        <div class="kinds">
          {#each s.all as k (k.id)}
            <label class="row kind"><input type="checkbox" checked={s.kinds.includes(k.id)} disabled={saving}
              onchange={(e) => toggleKind(k.id, e.currentTarget.checked)} />
              <span>{lockKindName(k.id)} <span class="faint mono">{k.patterns.join(" ")}</span></span></label>
          {/each}
        </div>
        <p class="faint small">{t("A project's .r3v.yaml can narrow this: turn locks off for it, or add and remove kinds of file.")}</p>
      {:else}
        <p class="faint small">{t("Turning it on: teammates on an older R3V must update before they can share.")}</p>
      {/if}
    {:else}
      <p class="small">{summary}</p>
      <p class="faint small">{t("Owners and admins change it.")}</p>
    {/if}
    {#if error}<p class="error small">{error}</p>{/if}
  </section>
{/if}

<style>
  section { margin-bottom: var(--sp-18); }
  h3 { font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--muted); margin: 0 0 var(--sp-8); }
  h4 { font-size: var(--fs-md); margin: var(--sp-10) 0 var(--sp-6); }
  .row { display: flex; align-items: center; gap: var(--sp-8); color: var(--text); font-size: var(--fs-base); margin: 0 0 var(--sp-4); }
  .row input { width: auto; }
  .kinds { display: flex; flex-direction: column; gap: var(--sp-2); margin-bottom: var(--sp-8); }
  .kind { font-size: var(--fs-md); }
  .mono { font-family: ui-monospace, monospace; font-size: var(--fs-xs); }
  .small { font-size: var(--fs-md); }
  section > p { margin: 0 0 var(--sp-8); }
  .error { color: var(--danger); user-select: text; }
</style>
