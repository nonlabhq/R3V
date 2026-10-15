<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "./i18n.svelte";
  import { api, errorText, type TeamSummary, type CloudStatus, type InvitationInfo } from "./api";

  // R3V-Cloud in "Join or create a team": sign in through the browser (the
  // account's teams come with it), then create a team or join one with an
  // invitation link. Nightly only, while it's built.
  let { onconnected }: { onconnected: (t: TeamSummary) => void } = $props();

  let status = $state<CloudStatus | null>(null);
  let waiting = $state(false); // for the browser
  let busy = $state(false);
  // (desktop/cloud.go: the beta's refusal, as cloud.ErrBetaOnly says it)
  const BETA_ONLY = "R3V Cloud is in a closed beta: teams are for invited people";
  const WAITLIST = "https://r3v.so/waitlist";
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

  // Joining: a team that moved in from its own storage brings its people
  // along, to be claimed: "who were you?" first (their versions, name and
  // look then theirs).
  let invite = $state<InvitationInfo | null>(null);
  let claim = $state("");
  async function join() {
    busy = true;
    error = "";
    try {
      const info = await api.CloudInvitationInfo(link);
      if (info && info.people.some((p) => !p.claimed)) {
        invite = info;
        claim = "";
        return;
      }
    } catch (e) {
      error = errorText(e);
      return;
    } finally {
      busy = false;
    }
    run(() => api.CloudJoinAs(link, ""));
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
  <p class="muted">{t("Signed in as {email}.", { email: status.email })}</p>

  <form onsubmit={(e) => { e.preventDefault(); run(() => api.CloudCreateTeam(name)); }}>
    <label for="cloud-name">{t("Create a team")}</label>
    <div class="row">
      <input id="cloud-name" bind:value={name} placeholder={t("Team name")} autocomplete="off" />
      <button type="submit" class="primary" disabled={busy || !name.trim()}>{t("Create")}</button>
    </div>
  </form>

  {#if invite}
    <div class="claim">
      <p><strong>{t("Who were you in {team}?", { team: invite.team })}</strong></p>
      <p class="muted">{t("The team moved to R3V Cloud with its people: pick yourself, and your versions, name and look are yours.")}</p>
      <div class="people" role="radiogroup" aria-label={t("Who were you?")}>
        {#each invite.people.filter((p) => !p.claimed) as p (p.id)}
          <label><input type="radio" name="claim" value={p.id} bind:group={claim} /> {p.name}</label>
        {/each}
        <label><input type="radio" name="claim" value="new" bind:group={claim} /> {t("I'm new to this team")}</label>
      </div>
      <div class="row actions"><span class="spacer"></span>
        <button onclick={() => (invite = null)}>{t("Back")}</button>
        <button class="primary" disabled={busy || !claim} onclick={() => run(() => api.CloudJoinAs(link, claim === "new" ? "" : claim))}>{t("Join")}</button>
      </div>
    </div>
  {:else}
  <form onsubmit={(e) => { e.preventDefault(); join(); }}>
    <label for="cloud-link">{t("Join with an invitation link")}</label>
    <div class="row">
      <input id="cloud-link" bind:value={link} placeholder="https://…/invite/…" autocomplete="off" spellcheck="false" />
      <button type="submit" disabled={busy || !link.trim()}>{t("Join")}</button>
    </div>
  </form>
  {/if}
{/if}
{#if error === BETA_ONLY}
  <!-- (the closed beta: teams are for invited people) -->
  <div class="beta">
    <p>{t("R3V Cloud is in a closed beta: making a team needs an invitation. You can still join teams you're invited to.")}</p>
    <button onclick={() => api.OpenURL(WAITLIST).catch(() => {})}>{t("Join the waitlist")}</button>
  </div>
{:else if error}<p class="error">{error}</p>{/if}

<style>
  .actions { margin-top: var(--sp-16); }
  form { margin-top: var(--sp-14); }
  form .row { gap: var(--sp-8); }
  form input { flex: 1; min-width: 0; }
  .error { color: var(--danger); user-select: text; }
  .beta { display: flex; align-items: center; gap: var(--sp-12); margin-top: var(--sp-10); padding: var(--sp-10) var(--sp-12);
    border-radius: var(--radius); background: var(--info-chip); }
  .beta p { flex: 1; margin: 0; font-size: var(--fs-md); }
  .claim { margin-top: var(--sp-14); }
  .people { display: flex; flex-direction: column; gap: var(--sp-6); margin: var(--sp-10) 0; }
  .people label { display: flex; align-items: center; gap: var(--sp-8); }
</style>
