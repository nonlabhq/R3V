<script lang="ts">
  import Modal from "$shared/Modal.svelte";
  import { t, tn } from "./i18n.svelte";
  import { api, errorText, type TeamProject } from "./api";
  import type { ArchiveInfo } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";

  // Archiving a project for the whole team (docs/design/archive.md): what
  // happens, in plain words, and what the copy here hasn't shared yet (then
  // the button says it archives all the same).
  let { teamId, p, onarchived, onclose }: {
    teamId: string;
    p: TeamProject;
    onarchived: () => void;
    onclose: () => void;
  } = $props();

  let info = $state<ArchiveInfo | null>(null);
  let error = $state("");
  let busy = $state(false);
  $effect(() => {
    api.ArchiveInfo(teamId, p.id).then((i) => (info = i)).catch((e) => (error = errorText(e)));
  });
  const notShared = $derived(!!info && (info.changes > 0 || info.unshared));

  async function archive() {
    busy = true;
    error = "";
    try {
      await api.ArchiveProject(teamId, p.id, notShared);
      onarchived();
    } catch (e) {
      error = errorText(e);
    } finally {
      busy = false;
    }
  }
</script>

<Modal title={t("Archive “{name}”?", { name: p.name })} {onclose}>
  <p>{t("It's hidden for everyone in the team. Nothing is deleted: its versions stay, and it can be brought back any time from the team's home, under Archived.")}</p>
  {#if p.root}<p class="hint">{t("The folder on this computer stays as it is; R3V stops checking it for updates.")}</p>{/if}
  {#if info && !info.onTeam}
    <p class="warn-text">{t("It isn't on the team yet: commit and share it first.")}</p>
  {:else if info && !info.canArchive}
    <p class="warn-text">{t("Only the team's admins can archive projects.")}</p>
  {/if}
  {#if info && info.changes > 0}
    <p class="warn-text">{tn(info.changes, "{n} change here isn't committed yet.", "{n} changes here aren't committed yet.")}</p>
  {/if}
  {#if info?.unshared}
    <p class="warn-text">{t("Some versions here aren't shared with the team yet.")}</p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
  {#snippet footer()}
    <button onclick={onclose}>{t("Cancel")}</button>
    <button class="primary" onclick={archive} disabled={busy || !info || !info.canArchive || !info.onTeam}>
      {notShared ? t("Archive anyway") : t("Archive")}</button>
  {/snippet}
</Modal>

<style>
  .hint { color: var(--muted); font-size: var(--fs-sm); }
  .warn-text { color: var(--warn); }
  .error { color: var(--danger); }
</style>
