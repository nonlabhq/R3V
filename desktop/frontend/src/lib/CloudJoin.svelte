<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "./i18n.svelte";
  import { api, errorText, type TeamSummary, type CloudStatus } from "./api";

  // R3V-Cloud in "Join or create a team": sign in through the browser (the
  // account's teams come with it), then create a team or join one with an
  // invitation link. Nightly only, while it's built.
  let { onconnected }: { onconnected: (t: TeamSummary) => void } = $props();

  let status = $state<CloudStatus | null>(null);
  let waiting = $state(false); // for the browser
  let busy = $state(false);
  let error = $state("");
  let name = $state("");
  let link = $state("");

  onMount(async () => {
    try {
      status = await api.CloudStatus();
    } catch (e) {
      error = errorText(e);
    }
  });

  async function signIn() {
    waiting = true;
    error = "";
    try {
      status = await api.CloudSignIn();
      // The account's teams are listed now: go to the one selected.
      const o = await api.Overview();
      const team = o?.teams.find((x) => x.id === o.currentTeam && x.hosted);
      if (team) onconnected(team);
    } catch (e) {
      if (waiting) error = errorText(e);
    } finally {
      waiting = false;
    }
  }

  function cancel() {
    waiting = false;
    api.CloudCancelSignIn();
  }

  async function signOut() {
    error = "";
    try {
      await api.CloudSignOut();
      status = await api.CloudStatus();
    } catch (e) {
      error = errorText(e);
    }
  }

  async function run(f: () => Promise<TeamSummary>) {
    busy = true;
    error = "";
    try {
      onconnected(await f());
    } catch (e) {
      error = errorText(e);
    } finally {
      busy = false;
    }
  }
</script>

{#if !status}
  <p class="faint">{t("Loading…")}</p>
{:else if !status.signedIn}
  <p class="muted">{t("Sign in to R3V-Cloud: nothing to set up, and your teams come with your account wherever you sign in.")}</p>
  {#if waiting}
    <p>{t("Finish signing in in your browser…")}</p>
    <div class="row actions"><span class="spacer"></span><button onclick={cancel}>{t("Cancel")}</button></div>
  {:else}
    <div class="row actions"><span class="spacer"></span>
      <button class="primary" onclick={signIn}>{t("Sign in with your browser")}</button></div>
  {/if}
{:else}
  <p class="muted">{t("Signed in as {email}.", { email: status.email })}
    <button class="link" onclick={signOut}>{t("Sign out")}</button></p>

  <form onsubmit={(e) => { e.preventDefault(); run(() => api.CloudCreateTeam(name)); }}>
    <label for="cloud-name">{t("Create a team")}</label>
    <div class="row">
      <input id="cloud-name" bind:value={name} placeholder={t("Team name")} autocomplete="off" />
      <button type="submit" class="primary" disabled={busy || !name.trim()}>{t("Create")}</button>
    </div>
  </form>

  <form onsubmit={(e) => { e.preventDefault(); run(() => api.CloudJoin(link)); }}>
    <label for="cloud-link">{t("Join with an invitation link")}</label>
    <div class="row">
      <input id="cloud-link" bind:value={link} placeholder="https://…/invite/…" autocomplete="off" spellcheck="false" />
      <button type="submit" disabled={busy || !link.trim()}>{t("Join")}</button>
    </div>
  </form>
{/if}
{#if error}<p class="error">{error}</p>{/if}

<style>
  .actions { margin-top: var(--sp-16); }
  form { margin-top: var(--sp-14); }
  form .row { gap: var(--sp-8); }
  form input { flex: 1; min-width: 0; }
  .error { color: var(--danger); user-select: text; }
  .link { border: none; background: none; padding: 0; color: var(--muted); text-decoration: underline; font-size: inherit; }
</style>
