<script lang="ts">
  import Modal from "$shared/Modal.svelte";
  import { t } from "./i18n.svelte";
  import type { DeletedInfo } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";

  // A share or commit stopped: the team deleted this project (moved it to
  // another team, or deleted it for everyone) while this copy still points
  // at it. Its versions stay here; nothing is done without asking: point
  // the copy at the team it moved to, bring it back (owners and admins),
  // or leave it.
  let { d, onfollow, onrestore, onclose }: {
    d: DeletedInfo;
    onfollow: () => void;
    onrestore: () => void;
    onclose: () => void;
  } = $props();

  const date = $derived(d.time ? new Date(d.time).toLocaleDateString() : "");
</script>

<Modal title={t("“{name}” isn't on {team} any more", { name: d.project, team: d.team })} {onclose}>
  <p>
    {date
      ? (d.by ? t("It was removed from {team} on {date} by {name}.", { team: d.team, date, name: d.by })
        : t("It was removed from {team} on {date}.", { team: d.team, date }))
      : t("It was removed from {team}.", { team: d.team })}
    {#if d.movedTo}{t("It moved to {team}; this copy still points at {old}.", { team: d.movedTo, old: d.team })}{/if}
  </p>
  <p>{t("Your versions stay on this computer.")}</p>
  {#if d.movedTo && d.movedToTeam}
    <p class="hint">{t("Point this copy at {team}: your versions not shared yet are shared there, after the versions shared there meanwhile.", { team: d.movedTo })}</p>
  {:else if d.movedTo}
    <p class="hint">{t("This computer hasn't joined {team}: join it, then point this copy at it.", { team: d.movedTo })}</p>
  {/if}
  {#if d.canRestore}
    <p class="hint">{t("Or bring it back to {team}, with its history, for everyone in it.", { team: d.team })}</p>
  {:else}
    <p class="hint">{t("Only the team's owners and admins can bring it back to {team}: ask one.", { team: d.team })}</p>
  {/if}
  {#snippet footer()}
    <button onclick={onclose}>{t("Cancel")}</button>
    {#if d.canRestore}
      <button class={d.movedToTeam ? "" : "primary"} onclick={onrestore}>{t("Bring it back to {team}", { team: d.team })}</button>
    {/if}
    {#if d.movedToTeam}
      <button class="primary" onclick={onfollow}>{t("Point this copy at {team}", { team: d.movedTo })}</button>
    {/if}
  {/snippet}
</Modal>

<style>
  .hint { color: var(--muted); font-size: var(--fs-sm); }
</style>
