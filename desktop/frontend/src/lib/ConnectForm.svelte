<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type TeamSummary } from "./api";

  // Join a team with its connection code, in onboarding and "Join/Create a
  // team".
  let { onconnected, submitLabel = "Connect" }: {
    onconnected: (t: TeamSummary) => void;
    submitLabel?: string;
  } = $props();

  let address = $state("");
  let error = $state("");
  let busy = $state(false);

  async function connect() {
    busy = true;
    error = "";
    try {
      onconnected(await api.ConnectTeam(address.trim()));
    } catch (e) {
      error = errorText(e);
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={(e) => { e.preventDefault(); connect(); }}>
  <label for="addr">{t("Connection code")}</label>
  <input id="addr" bind:value={address} placeholder={t("r3v-s3:…")} autocomplete="off" spellcheck="false" />
  {#if error}<p class="error">{error}</p>{/if}
  <div class="row actions">
    <span class="spacer"></span>
    <button type="submit" class="primary" disabled={!address.trim() || busy}>{busy ? "Connecting…" : submitLabel}</button>
  </div>
</form>

<style>
  .actions { margin-top: 16px; }
  .error { color: var(--danger); user-select: text; }
</style>
