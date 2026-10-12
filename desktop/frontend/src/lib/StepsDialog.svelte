<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import type { Session } from "./api";
  import Modal from "./Modal.svelte";
  import StepsList from "./StepsList.svelte";

  // Steps to look back at (a version's, a workspace's): each session's
  // prompts and steps, to compare; going back is for the work you're on.
  let { root, title, sessions, onclose }: {
    root: string; // the copy asked (a session kept by another says its own)
    title: string;
    sessions: Session[];
    onclose: () => void;
  } = $props();
</script>

<Modal {title} width={620} {onclose}>
  {#each sessions as s (s.copy + s.id)}
    <section>
      {#if sessions.length > 1}<p class="faint small">{tn(s.steps, "{n} step", "{n} steps")}{s.copy ? ` · ${s.copy}` : ""}</p>{/if}
      <StepsList root={s.copy || root} session={s} />
    </section>
  {:else}
    <p class="faint">{t("No steps kept for this on this computer.")}</p>
  {/each}
  {#snippet footer()}<button onclick={onclose}>{t("Close")}</button>{/snippet}
</Modal>

<style>
  section + section { border-top: var(--border-width) solid var(--line); margin-top: var(--sp-8); padding-top: var(--sp-6); }
  .small { font-size: var(--fs-sm); margin: 0; }
</style>
