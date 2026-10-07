<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "./i18n.svelte";
  import { api, type TeamSummary } from "./api";
  import ConnectForm from "./ConnectForm.svelte";
  import StorageSetup from "./StorageSetup.svelte";
  import CloudJoin from "./CloudJoin.svelte";

  // Join a team with a connection code, or create one on your own storage;
  // on Nightly, also R3V-Cloud (hosted teams).
  let { onconnected }: { onconnected: (t: TeamSummary) => void } = $props();
  let mode = $state<"join" | "create" | "cloud">("join");
  let created = $state(false); // showing the new team's code: no switching back
  let cloud = $state(false);
  onMount(() => {
    api.CloudStatus().then((s) => (cloud = !!s?.available)).catch(() => {});
  });
</script>

{#if !created}
<div class="seg" role="tablist">
  <button role="tab" class:on={mode === "join"} aria-selected={mode === "join"} onclick={() => (mode = "join")}>{t("Join a team")}</button>
  <button role="tab" class:on={mode === "create"} aria-selected={mode === "create"} onclick={() => (mode = "create")}>{t("Create a team")}</button>
  {#if cloud}
    <button role="tab" class:on={mode === "cloud"} aria-selected={mode === "cloud"} onclick={() => (mode = "cloud")}>R3V-Cloud</button>
  {/if}
</div>
{/if}

{#if mode === "join"}
  <p class="muted">{t("Paste the connection code from whoever set up your team.")}
    <button class="link" onclick={() => (mode = "create")}>{t("Setting it up yourself?")}</button></p>
  <ConnectForm {onconnected} />
{:else if mode === "create"}
  <StorageSetup {onconnected} oncreated={() => (created = true)} />
{:else}
  <CloudJoin {onconnected} />
{/if}

<style>
  .seg { display: flex; margin: 0 0 var(--sp-14); }
  .seg button { flex: 1; border-radius: 0; padding: var(--sp-6) var(--sp-12); }
  .seg button:first-child { border-radius: var(--radius-lg) 0 0 var(--radius-lg); }
  .seg button:last-child { border-radius: 0 var(--radius-lg) var(--radius-lg) 0; margin-left: -1px; }
  .seg button:not(:first-child):not(:last-child) { margin-left: -1px; }
  .seg button.on { background: var(--accent); color: var(--accent-ink); border-color: var(--accent); }
  .link { border: none; background: none; padding: 0; color: var(--muted); text-decoration: underline; font-size: inherit; }
</style>
