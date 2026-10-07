<script lang="ts">
  import { onMount } from "svelte";
  import { t, tn } from "./i18n.svelte";
  import { api, errorText, formatBytes, type TeamSummary, type TeamMoveEstimate, type TeamMoveState } from "./api";
  import { Events } from "@wailsio/runtime";
  import Modal from "./Modal.svelte";

  // Moving a team from its own storage to R3V Cloud (docs/design/moving.md):
  // what it means (sizes, people), where to, which projects, a read-only
  // key for the service; then the copy in the background while everyone
  // works on, and once it's copied, finishing (a short freeze). Nightly.
  let { team, teams, reload, onclose }: {
    team: TeamSummary;      // the one moving (on its own storage)
    teams: TeamSummary[];   // where it can go: hosted teams here
    reload: () => Promise<void>;
    onclose: () => void;
  } = $props();

  let est = $state<TeamMoveEstimate | null>(null);
  let st = $state<TeamMoveState | null>(null);
  let error = $state("");
  let busy = $state(false);
  let to = $state("");
  let picked = $state<Record<string, boolean>>({});
  let accessKey = $state("");
  let secretKey = $state("");
  let region = $state("");
  let hostedTeams = $derived(teams.filter((x) => x.hosted && !x.signedOut && !x.noAccess));

  async function refresh() {
    try {
      st = await api.TeamMoveState(team.id);
    } catch (e) {
      error = errorText(e);
    }
  }
  onMount(() => {
    refresh().then(async () => {
      if (st?.phase) return; // under way: its progress
      try {
        est = await api.EstimateTeamMove(team.id);
        picked = Object.fromEntries(est!.projects.map((p) => [p.id, true]));
      } catch (e) {
        error = errorText(e);
      }
    });
    const off = Events.On("team-move", (ev) => { if (ev.data === team.id) refresh(); });
    const poll = setInterval(() => { if (st?.phase === "copying" || st?.phase === "finishing") refresh(); }, 3000);
    return () => { off(); clearInterval(poll); };
  });

  let chosen = $derived(est?.projects.filter((p) => picked[p.id]) ?? []);
  let chosenBytes = $derived(chosen.reduce((n, p) => n + p.bytes, 0));

  async function act(f: () => Promise<unknown>) {
    busy = true;
    error = "";
    try {
      await f();
      await refresh();
    } catch (e) {
      error = errorText(e);
    } finally {
      busy = false;
    }
  }
  const start = () => act(() => api.StartTeamMove(team.id, to, chosen.map((p) => p.id), accessKey, secretKey, region));
  const finish = () => act(() => api.FinishTeamMove(team.id));
  const cancel = () => act(() => api.CancelTeamMove(team.id));
  let pct = $derived(st && st.bytes ? Math.round((100 * st.bytesDone) / st.bytes) : 0);
</script>

<Modal title={t("Move {team} to R3V Cloud", { team: team.name })} width={600} {onclose}>
  {#if st?.phase === "done"}
    <p><strong>{t("{team} is on R3V Cloud now.", { team: team.name })}</strong></p>
    <p class="muted">{t("Invite your teammates there (People, in the new team's settings). When they join, they pick who they were, and their projects come along as they are.")}</p>
    <p class="muted">{t("Your old storage is kept as it was: delete it once you're happy with the move (R3V never does).")}</p>
  {:else if st?.phase}
    {#if st.phase === "planning"}
      <p>{t("Listing what each project needs…")}</p>
    {:else if st.phase === "copying" && !st.copied}
      <p>{t("R3V Cloud is copying the team's files. Everyone can keep working, and you can close R3V: it goes on.")}</p>
    {:else if st.phase === "copying"}
      <p><strong>{t("Copied.")}</strong> {t("Finishing stops sharing to the old team for a few minutes, copies what was shared since, then moves everyone's work over.")}</p>
    {:else}
      <p>{t("Finishing: the old team takes no shares meanwhile (versions are kept and shared afterwards).")}</p>
    {/if}
    {#if st.bytes}
      <div class="bar"><div style:width="{pct}%"></div></div>
      <p class="faint">{t("{done} of {total}", { done: formatBytes(st.bytesDone), total: formatBytes(st.bytes) })} · {tn(st.items, "{n} file", "{n} files", { n: st.items })}</p>
    {/if}
    {#if st.failed}<p class="warn">{tn(st.failed, "{n} file couldn't be copied: Finish tries it again.", "{n} files couldn't be copied: Finish tries them again.", { n: st.failed })}</p>{/if}
    {#if st.error}<p class="err">{st.error}</p>{/if}
  {:else if !est}
    {#if !error}<p>{t("Reading the team's projects…")}</p>{/if}
  {:else}
    <p class="muted">{t("R3V Cloud copies the team's history from its storage itself, in the background; you don't download or upload anything.")}</p>
    <h3>{t("What moves")}</h3>
    <ul class="projects">
      {#each est.projects as p (p.id)}
        <li><label><input type="checkbox" bind:checked={picked[p.id]} disabled={busy} />
          <span class="name">{p.name}</span>
          <span class="faint">{tn(p.versions, "{n} version", "{n} versions", { n: p.versions })} · {formatBytes(p.bytes)}</span></label></li>
      {/each}
    </ul>
    <p class="hint">{t("On R3V Cloud: {hosted} for these projects; your storage holds {storage} now (each project keeps its own copy of a file several of them use).", { hosted: formatBytes(chosenBytes), storage: formatBytes(est.storage) })}
      {#if est.people}{tn(est.people, "{n} person comes along, to be invited.", "{n} people come along, to be invited.", { n: est.people })}{/if}</p>

    <h3>{t("To")}</h3>
    {#if hostedTeams.length}
      <select bind:value={to} disabled={busy} aria-label={t("Team on R3V Cloud")}>
        <option value="" disabled>{t("Pick a team on R3V Cloud")}</option>
        {#each hostedTeams as x (x.id)}<option value={x.id}>{x.name}</option>{/each}
      </select>
    {:else}
      <p class="hint">{t("Sign in to R3V Cloud and create the team there first (Join or create a team): it shows here then.")}</p>
    {/if}

    <h3>{t("A read-only key for R3V Cloud")}</h3>
    {#if est.provider === "r2"}
      <p class="hint">{t("In Cloudflare: R2 › Manage API tokens › Create API token, with \"Object Read only\" for the bucket {bucket}.", { bucket: est.bucket })}</p>
    {:else if est.provider === "aws"}
      <p class="hint">{t("In AWS: an access key of a user allowed only s3:GetObject and s3:ListBucket on the bucket {bucket}.", { bucket: est.bucket })}</p>
    {:else}
      <p class="hint">{t("A key that can only read the bucket {bucket}.", { bucket: est.bucket })}</p>
    {/if}
    <div class="keys">
      <input bind:value={accessKey} placeholder={t("Access key")} aria-label={t("Access key")} autocomplete="off" spellcheck="false" disabled={busy} />
      <input bind:value={secretKey} type="password" placeholder={t("Secret key")} aria-label={t("Secret key")} autocomplete="off" disabled={busy} />
      {#if est.provider === "aws"}<input bind:value={region} placeholder="eu-central-1" aria-label={t("Region")} disabled={busy} />{/if}
    </div>
    <p class="hint">{t("Only R3V Cloud gets it, for the move, and forgets it after. The team's own key never leaves this computer.")}</p>
  {/if}
  {#if error}<p class="err" role="alert">{error}</p>{/if}

  {#snippet footer()}
    {#if st?.phase && st.phase !== "done"}
      <button onclick={cancel} disabled={busy || st.phase === "finishing"}>{t("Give up the move")}</button>
      <span class="grow"></span>
      <button onclick={onclose}>{t("Close")}</button>
      {#if st.phase === "copying" && st.copied}
        <button class="primary" onclick={finish} disabled={busy}>{t("Finish the move")}</button>
      {/if}
    {:else if st?.phase === "done"}
      <button class="primary" onclick={async () => { await reload(); onclose(); }}>{t("Done")}</button>
    {:else}
      <button onclick={onclose}>{t("Cancel")}</button>
      <button class="primary" onclick={start} disabled={busy || !est || !to || !chosen.length || !accessKey.trim() || !secretKey.trim()}>
        {busy ? t("Starting…") : t("Start copying")}</button>
    {/if}
  {/snippet}
</Modal>

<style>
  h3 { margin: var(--sp-14) 0 var(--sp-6); font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); font-weight: var(--fw-semibold); }
  .muted { color: var(--muted); }
  .hint { color: var(--faint); font-size: var(--fs-sm); margin: var(--sp-6) 0 0; }
  .faint { color: var(--faint); font-size: var(--fs-sm); }
  .projects { list-style: none; margin: 0; padding: 0; max-height: 200px; overflow: auto; }
  .projects label { display: flex; align-items: center; gap: var(--sp-8); padding: var(--sp-4) 0; }
  .projects .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .keys { display: flex; flex-direction: column; gap: var(--sp-6); }
  select { width: 100%; }
  .bar { height: 6px; border-radius: 3px; background: var(--line); overflow: hidden; margin: var(--sp-10) 0 var(--sp-6); }
  .bar > div { height: 100%; background: var(--accent); transition: width .3s; }
  .warn { color: var(--warn); font-size: var(--fs-sm); }
  .err { color: var(--danger); font-size: var(--fs-sm); user-select: text; }
  .grow { flex: 1; }
</style>
