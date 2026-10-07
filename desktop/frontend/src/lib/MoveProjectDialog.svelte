<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type Progress, type TeamSummary } from "./api";
  import Modal from "./Modal.svelte";
  import ProgressBar from "./ProgressBar.svelte";

  // Moving a project to another team (or copying it there): its whole
  // history goes straight from one team's storage to the other's; the
  // folder here is untouched. Moved, the first team no longer has it: its
  // people keep their copies, as projects of their own.
  let { root, name, team, teams, progress, onmoved, onclose }: {
    root: string;
    name: string;
    team: TeamSummary;     // the one it is in
    teams: TeamSummary[];  // where it can go
    progress: Progress | null;
    onmoved: (teamId: string, copied: boolean) => void;
    onclose: () => void;
  } = $props();

  let to = $state("");
  let copy = $state(false);
  let busy = $state(false);
  let cancelling = $state(false);
  let error = $state("");
  let choices = $derived(teams.filter((x) => x.id !== team.id && !x.noAccess && !x.signedOut));
  let target = $derived(choices.find((x) => x.id === to));

  async function move() {
    if (!target) return;
    busy = true;
    error = "";
    try {
      await api.MoveProject(root, target.id, copy);
      onmoved(target.id, copy);
    } catch (e) {
      error = cancelling ? t("Stopped. Nothing changed for either team; moving again goes on from where it stopped.") : errorText(e);
    } finally {
      busy = false;
      cancelling = false;
    }
  }
  function cancel() {
    cancelling = true;
    api.CancelSave(root).catch(() => {});
  }
</script>

<Modal title={t("Move “{name}” to another team", { name })} width={520} onclose={() => { if (!busy) onclose(); }} escCloses={!busy}>
  <p class="muted">{t("Its whole history goes to the other team, straight from one team's storage to the other's. The folder on this computer stays as it is.")}</p>
  {#if choices.length}
    <div class="teams" role="radiogroup" aria-label={t("Team")}>
      {#each choices as x (x.id)}
        <label class="team" class:on={to === x.id}>
          <input type="radio" name="to" value={x.id} bind:group={to} disabled={busy} />
          <span>{x.name}</span>
          {#if x.hosted}<span class="faint">R3V Cloud</span>{/if}
        </label>
      {/each}
    </div>
    <label class="copy">
      <input type="checkbox" bind:checked={copy} disabled={busy} />
      {t("Keep it in {team} too (copy it instead)", { team: team.name })}
    </label>
    {#if target}
      <p class="hint">
        {copy ? t("Both teams will have it; this folder stays with {team}.", { team: team.name })
          : t("{team} won't have it any more: its people keep their copies, as projects on their own computer. This folder goes with {to}.", { team: team.name, to: target.name })}
      </p>
    {/if}
  {:else}
    <p class="hint">{t("Join or set up the other team first: it shows here then.")}</p>
  {/if}
  {#if busy}<div class="progress"><ProgressBar p={progress} team={target?.name} waiting={t("Moving…")} /></div>{/if}
  {#if error}<p class="err" role="alert">{error}</p>{/if}

  {#snippet footer()}
    {#if busy}
      <button onclick={cancel} disabled={cancelling}>{cancelling ? t("Cancelling…") : t("Cancel")}</button>
    {:else}
      <button onclick={onclose}>{t("Cancel")}</button>
      <button class="primary" onclick={move} disabled={!target}>{copy ? t("Copy") : t("Move")}</button>
    {/if}
  {/snippet}
</Modal>

<style>
  .muted { color: var(--muted); margin: 0 0 var(--sp-12); }
  .teams { display: flex; flex-direction: column; gap: var(--sp-4); margin-bottom: var(--sp-12); }
  .team { display: flex; align-items: center; gap: var(--sp-8); padding: var(--sp-6) var(--sp-10); border-radius: var(--radius);
    border: var(--border-width) solid var(--line); cursor: pointer; }
  .team.on { border-color: var(--accent); background: var(--accent-soft); }
  .team .faint { margin-left: auto; color: var(--faint); font-size: var(--fs-sm); }
  .copy { display: flex; align-items: center; gap: var(--sp-8); font-size: var(--fs-md); }
  .hint { color: var(--faint); font-size: var(--fs-sm); margin: var(--sp-8) 0 0; }
  .progress { margin-top: var(--sp-12); }
  .err { color: var(--danger); font-size: var(--fs-sm); margin: var(--sp-8) 0 0; }
</style>
