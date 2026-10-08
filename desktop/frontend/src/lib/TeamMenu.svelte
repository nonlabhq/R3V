<script lang="ts">
  import { t as tr, tn } from "./i18n.svelte"; // t is a team here
  import { api, errorText, type Overview, type TeamSummary } from "./api";
  import { toast } from "./notify.svelte";
  import Modal from "./Modal.svelte";
  import JoinOrCreate from "./JoinOrCreate.svelte";
  import IdentityForm from "./IdentityForm.svelte";

  // settingsFor: the team whose settings are open (the ⚙ of a team; the app
  // shows them, so a project's header can open them too).
  let { overview, reload, settingsFor = $bindable(null) }: {
    overview: Overview; reload: () => Promise<void>; settingsFor?: TeamSummary | null;
  } = $props();

  let open = $state(false);
  let connecting = $state(false);
  // Who you are in a team (after connecting, or to rename yourself).
  let identityFor = $state<TeamSummary | null>(null);

  let current = $derived(overview.teams.find((t) => t.id === overview.currentTeam));
  // Sharing your setup not chosen yet (a member from before the option, or
  // back on a team): asked once.
  let answered = $state<Record<string, boolean>>({});
  let shareAsk = $derived(!identityFor && !connecting && current?.askShareSetup && !answered[current.id] ? current : null);
  async function answerShare(team: TeamSummary, on: boolean) {
    answered[team.id] = true;
    try {
      await api.SetShareSetup(team.id, on);
      if (on) toast(tr("Your setup is shared with {team}", { team: team.name }), "ok");
      await reload();
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
  // No one has backed up the team lately: suggested, until put off for a
  // week (asked again when the team settings close: one may be set up).
  let remindBackup = $state(false);
  $effect(() => {
    const id = current?.memberId && current.isStorage && !settingsFor ? current.id : "";
    remindBackup = false;
    if (id) api.BackupReminder(id).then((r) => { if (current?.id === id) remindBackup = r; }).catch(() => {});
  });
  async function hushBackup(team: TeamSummary) {
    remindBackup = false;
    try {
      await api.HushBackupReminder(team.id);
    } catch (e) {
      toast(errorText(e), "error");
    }
  }

  // A hosted team's computer signed out: signing in again from here.
  let signingIn = $state(false);
  async function signIn() {
    signingIn = true;
    try {
      await api.CloudSignIn();
      await reload();
    } catch (e) {
      if (signingIn) toast(errorText(e), "error"); // (not when cancelled)
    } finally {
      signingIn = false;
    }
  }
  // The browser tab left alone: stop waiting for it.
  function cancelSignIn() {
    signingIn = false;
    api.CloudCancelSignIn();
  }

  async function select(id: string) {
    open = false;
    try {
      await api.SelectTeam(id);
      await reload();
    } catch (e) {
      toast(errorText(e), "error");
    }
  }

  async function connected(t: TeamSummary) {
    connecting = false;
    toast(tr("Connected to {team}", { team: t.name }), "ok");
    await reload();
    if (!t.memberId) {
      identityFor = t;
      offerAfterName = t;
    } else {
      offerReconnect(t);
    }
  }

  async function identitySaved(t: TeamSummary, renamed: boolean) {
    identityFor = null;
    await reload();
    toast(tr(renamed ? "You're “{name}” in {team} — on all your versions" : "You're “{name}” in {team}", { name: t.memberName, team: t.name }), "ok");
    if (offerAfterName) {
      const joined = offerAfterName;
      offerAfterName = null;
      offerReconnect(joined);
    }
  }

  // Projects of the team already on this computer (e.g. from before it
  // disconnected): offered for reconnecting, ticked by default.
  let offerAfterName: TeamSummary | null = null;
  let found = $state<{ team: TeamSummary; projects: { root: string; name: string; on: boolean }[] } | null>(null);
  let reconnecting = $state(false);

  async function offerReconnect(t: TeamSummary) {
    try {
      const ps = (await api.TeamProjectsHere(t.id)) ?? [];
      if (ps.length) found = { team: t, projects: ps.map((p) => ({ ...p, on: true })) };
    } catch {
      // not reachable now: they can still be added by hand
    }
  }

  async function reconnect() {
    if (!found) return;
    const f = found;
    const roots = f.projects.filter((p) => p.on).map((p) => p.root);
    reconnecting = true;
    try {
      await api.ReconnectProjects(f.team.id, roots);
      found = null;
      await reload();
      toast(tn(roots.length, "Reconnected {n} project to {team}", "Reconnected {n} projects to {team}", { team: f.team.name }), "ok");
    } catch (e) {
      toast(errorText(e), "error", 9000);
    } finally {
      reconnecting = false;
    }
  }
  // The team moved to R3V Cloud and this computer joined it there: its
  // projects follow (the same folders, nothing downloaded).
  let following = $state(false);
  async function follow() {
    const from = current!, to = current!.movedToTeam;
    following = true;
    try {
      await api.FollowMovedTeam(from.id, to);
      await api.SelectTeam(to);
      await reload();
      toast(tr("Your projects are on R3V Cloud now, as they were."), "ok");
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      following = false;
    }
  }
</script>

<svelte:window onclick={(e) => { if (open && !(e.target as HTMLElement).closest(".team-menu")) open = false; }} />

<div class="team-menu">
  <!-- the team (a click lists the others), and its settings right there -->
  <div class="current">
    <button class="switch" onclick={() => (open = !open)} title={current?.address ?? ""} aria-haspopup="menu" aria-expanded={open}>
      <span class="label">{tr("Team")}</span>
      <span class="name">{current?.name ?? tr("No team")}</span>
    </button>
    {#if current}
      <button class="ghost settings" title={tr("Team settings: names, connection code, keys")} aria-label={tr("Team settings: names, connection code, keys")}
        onclick={() => { open = false; settingsFor = current!; }}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
          <circle cx="12" cy="12" r="3"/>
        </svg>
      </button>
    {/if}
  </div>
  {#if current?.hosted && current.signedOut}
    <button class="who" disabled={signingIn} onclick={signIn}>
      ⚠ {signingIn ? tr("Finish signing in in your browser…") : tr("Signed out of R3V-Cloud: sign in to share and update")}
    </button>
    {#if signingIn}<button class="who" onclick={cancelSignIn}>{tr("Cancel")}</button>{/if}
  {/if}
  {#if current?.hosted && current.noAccess && !current.signedOut}
    <!-- the account signed in isn't in it (any more): the person decides -->
    <p class="who noaccess">⚠ {tr("The account signed in isn't in this team (any more). Its projects stay here as they are: remove the team in its settings to keep them as local projects.")}</p>
  {/if}
  {#if current?.moving}
    <p class="who moved">↗ {tr("{team} is moving to R3V Cloud: versions you commit are kept here and shared once it's there.", { team: current.name })}</p>
  {:else if current?.movedTo}
    <!-- moved to R3V Cloud: join it there, and the projects come along -->
    <div class="who moved">
      ↗ {tr("{team} moved to R3V Cloud.", { team: current.name })}
      {#if current.movedToTeam}
        <button class="go" disabled={following} onclick={follow}>{tr("Bring my projects there")}</button>
      {:else}
        {tr("Ask a teammate for an invitation to it: when you join, your projects come along as they are.")}
      {/if}
    </div>
  {/if}
  {#if current && !current.memberId}
    <button class="who" onclick={() => (identityFor = current!)}
      title={tr("Versions you commit here show this name, for everyone in the team")}>
      ☺ {tr("Choose your name in {team}", { team: current.name })}
    </button>
  {/if}
  {#if current?.backupFailing}
    <button class="backup warn" onclick={() => (settingsFor = current!)}>
      ⚠ {tr("Backups of {team} keep failing on this computer. Have a look", { team: current.name })} ›</button>
  {:else if current && remindBackup}
    <div class="backup">
      <button class="go" onclick={() => (settingsFor = current!)}>
        <span>⛁ {tr("No one backs up {team} yet", { team: current.name })}</span>
        <span class="faint">{tr("Set up a backup to a drive or NAS")} ›</span>
      </button>
      <button class="ghost x" title={tr("Remind me in a week")} onclick={() => hushBackup(current!)}>✕</button>
    </div>
  {/if}
  {#if open}
    <div class="menu surface-menu" role="menu">
      {#each overview.teams as t (t.id)}
        <div class="team-row">
          <button class="item" onclick={() => select(t.id)}>
            <span class="check">{t.id === overview.currentTeam ? "✓" : ""}</span>
            <span class="tname">{t.name}</span>
          </button>
          <button class="gear" title={tr("Team settings: names, connection code, keys")}
            onclick={() => { open = false; settingsFor = t; }}>⚙</button>
        </div>
      {/each}
      {#if overview.teams.length}<div class="sep"></div>{/if}
      <button class="item" onclick={() => { open = false; connecting = true; }}>
        <span class="check">+</span>{tr("Join/Create a Team…")}
      </button>
    </div>
  {/if}
</div>

{#if shareAsk}
  {@const t = shareAsk}
  <Modal title={tr("Share your setup with {team}?", { team: t.name })} onclose={() => answerShare(t, false)}>
    <p>{tr("Your Ableton Live version and the names of your plugins and packs (never files), so a project check can tell who can open a project.")}</p>
    <p class="muted">{tr("You can change this in the team's settings.")}</p>
    {#snippet footer()}
      <button onclick={() => answerShare(t, false)}>{tr("Not now")}</button>
      <button class="primary" onclick={() => answerShare(t, true)}>{tr("Share")}</button>
    {/snippet}
  </Modal>
{/if}

{#if identityFor}
  {@const t = identityFor}
  <Modal title={t.memberId ? tr("Your name in {team}", { team: t.name }) : tr("Who are you in {team}?", { team: t.name })} onclose={() => (identityFor = null)}>
    <IdentityForm team={t} suggested={overview.author} submitLabel={tr("Save")} askShare
      onsaved={(saved) => identitySaved(saved, !!t.memberId && saved.memberName !== t.memberName)} />
  </Modal>
{/if}

{#if connecting}
  <Modal title={tr("Join or create a team")} onclose={() => (connecting = false)} width={640} backdropCloses={false}>
    <JoinOrCreate onconnected={connected} />
  </Modal>
{/if}

{#if found}
  {@const f = found}
  <Modal title={tr("Projects of {team} on this computer", { team: f.team.name })} onclose={() => (found = null)} backdropCloses={false}>
    <p class="muted">{tr("These projects on this computer belong to {team}. Reconnect them to share versions with the team again; their history is kept.", { team: f.team.name })}</p>
    <ul class="found">
      {#each f.projects as p (p.root)}
        <li><label><input type="checkbox" bind:checked={p.on} />
          <span><strong>{p.name}</strong><span class="faint small mono">{p.root}</span></span></label></li>
      {/each}
    </ul>
    {#snippet footer()}
      <button onclick={() => (found = null)}>{tr("Not now")}</button>
      <button class="primary" disabled={reconnecting || !f.projects.some((p) => p.on)} onclick={reconnect}>
        {reconnecting ? tr("Reconnecting…") : tr("Reconnect selected")}</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .noaccess { margin: 0; cursor: default; white-space: normal; line-height: 1.35; }
  .team-menu { position: relative; margin-bottom: var(--sp-10); }
  .backup { display: flex; align-items: flex-start; gap: var(--sp-4); width: 100%; margin-top: var(--sp-6); padding: var(--sp-6) var(--sp-8); border-radius: var(--radius);
    background: var(--panel); font-size: var(--fs-sm); text-align: left; line-height: 1.4; }
  .backup.warn { display: block; background: var(--warn-bg); color: var(--warn); }
  .backup .go { flex: 1; display: flex; flex-direction: column; gap: var(--sp-2); padding: 0; background: none; text-align: left; font-size: var(--fs-sm); color: var(--text); }
  .backup .x { padding: 0 var(--sp-4); line-height: 16px; color: var(--muted); }
  .current { display: flex; align-items: stretch; background: var(--panel); border-radius: var(--radius-lg); }
  .switch {
    flex: 1; min-width: 0; display: flex; flex-direction: column; text-align: left; padding: var(--sp-8) var(--sp-10);
    background: transparent; border-color: transparent; border-radius: var(--radius-lg);
  }
  .switch:hover:not(:disabled) { background: var(--hover); border-color: transparent; }
  .label { font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); }
  .name { font-weight: var(--fw-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .settings { flex: none; align-self: center; margin-right: var(--sp-6); padding: var(--sp-6); line-height: 0; color: var(--muted); }
  .settings:hover:not(:disabled) { color: var(--text); }
  .settings svg { width: 18px; height: 18px; }
  .menu {
    position: absolute; top: calc(100% + 4px); left: 0; right: -60px; z-index: var(--z-menu); padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop);
  }
  .item { display: flex; align-items: center; gap: var(--sp-8); width: 100%; border: none; background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left; }
  .item:hover { background: var(--hover); }
  .moved { color: var(--accent); font-size: var(--fs-sm); line-height: 1.4; }
  .moved .go { display: block; margin-top: var(--sp-4); font-size: var(--fs-sm); }
  .team-row { display: flex; align-items: center; }
  .team-row .item { flex: 1; min-width: 0; }
  .gear { flex: none; border: none; background: transparent; color: var(--faint); padding: var(--sp-4) var(--sp-8); border-radius: var(--radius); }
  .gear:hover { color: var(--text); background: var(--hover); }
  .check { width: 14px; color: var(--ok); }
  .tname { flex: 1; }
  .small { font-size: var(--fs-sm); }
  .who {
    width: 100%; margin-top: var(--sp-6); padding: var(--sp-4) var(--sp-10); font-size: var(--fs-md); text-align: left;
    background: var(--warn-bg); color: var(--warn); border: var(--border-width) solid var(--warn-line); border-radius: var(--radius-lg);
  }
  .sep { height: 1px; background: var(--line); margin: var(--sp-6) 0; }
  .found { list-style: none; padding: 0; margin: var(--sp-12) 0 0; display: flex; flex-direction: column; gap: var(--sp-8); }
  .found label { display: flex; gap: var(--sp-10); align-items: flex-start; margin: 0; color: var(--text); font-size: var(--fs-base); }
  .found input { width: auto; margin-top: var(--sp-4); }
  .found label > span { flex: 1; min-width: 0; }
  .found .mono { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
