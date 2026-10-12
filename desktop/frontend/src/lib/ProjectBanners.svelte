<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { ago, titleOf, type State, type Progress, type RuleSuggestion as Suggestion } from "./api";
  import Tx from "./Tx.svelte";
  import RuleSuggestion from "./RuleSuggestion.svelte";

  // What a project's page says above its tabs that needs doing, most
  // urgent first: not shared yet, a sync service's folder, a switch that
  // didn't finish, other tools' projects found, an older version, missing
  // samples, broken rules. Each says what to do; the page does it.
  // (Transfers and the team's new versions are in the title bar's notice
  // slot: NoticeSlot.)
  let { st, busy, progress, restorable, missingSamples, onshare, onrecover, onpreset, onbranchhere, onlatest,
    oncombine, onnewbranch, onkeep, onupdate, onrestore, onopenrules, loadError = "",
    onrestorebranch, onswitchmain, onlockson, onlocksnotnow }: {
    st: State;
    busy: string;
    progress: Progress | null;
    restorable: number; // missing samples R3V has copies of
    missingSamples: number;
    onshare: () => void; // share the versions of a project that joined a team
    onrecover: () => void; // put back the files of a switch that didn't finish
    onpreset: (s: Suggestion, preset: string) => void;
    onbranchhere: () => void; // commit the changes on an older version on a new branch
    onlatest: () => void;
    oncombine: () => void; // commit on an older version, combined with the latest
    onnewbranch: () => void;
    onkeep: () => void; // make the older version the latest (no team)
    onupdate: () => void;
    onrestore: () => void; // the missing samples
    onopenrules: () => void;
    loadError?: string; // the project couldn't be read again just now (what is shown is from before)
    onrestorebranch?: () => void; // the branch you are on was deleted: bring it back
    onswitchmain?: () => void;
    onlockson?: () => void; // turn file locking on for the team (offered once)
    onlocksnotnow?: () => void;
  } = $props();

  // The warning about OneDrive & co., once understood, stays away (per project).
  let cloudOk = $state<Record<string, boolean>>({});
  $effect.pre(() => {
    const r = st.root;
    try { cloudOk[r] = localStorage.getItem(`r3v.cloudOk:${r}`) === "1"; } catch { /* shown */ }
  });
  function okCloud(r: string) {
    cloudOk[r] = true;
    try { localStorage.setItem(`r3v.cloudOk:${r}`, "1"); } catch { /* shown again next time */ }
  }
  let spotsDismissed = $state(-1); // the count "Later" was said to
  // A download that didn't finish (the connection lost, R3V closed): no
  // version here yet, the team has some.
  let notDownloaded = $derived(!!st.remoteUrl && !st.head && st.incoming.length > 0);
</script>


{#if notDownloaded && !busy && !progress}
  <div class="banner warn">
    <div>{t("The download didn't finish (the connection was lost or R3V closed): get the files to finish it.")}</div>
    <button class="primary" onclick={onupdate}>{t("Finish downloading")}</button>
  </div>
{:else if st.remoteUrl && !st.head && !busy && !progress}
  <div class="banner info">
    <div>{t("Not shared with {team} yet. Look through the files and ignore the folders or files you don't need (right-click › Ignore), then commit a first version to share it.", { team: st.teamName || t("the team") })}</div>
  </div>
{:else if st.remoteUrl && st.unshared && !busy && !progress}
  <div class="banner info">
    <div>{t("Not shared with {team} yet: its versions are on this computer only.", { team: st.teamName || t("the team") })}</div>
    <button class="primary" onclick={onshare}>{t("Share now")}</button>
  </div>
{/if}

{#if st.locksOffer && onlockson && !busy && !progress}
  <div class="banner info">
    <div>{t("Files like these can't be merged: when two people change the same one, one of them loses their work. With file locking, {team} sees who is working on which file, and a change to a file someone else holds waits until they're done.", { team: st.teamName || t("the team") })}</div>
    {#if onlocksnotnow}<button onclick={onlocksnotnow}>{t("Not now")}</button>{/if}
    <button class="primary" onclick={onlockson}>{t("Turn on file locking")}</button>
  </div>
{/if}

{#if st.branchGone && !busy && !progress}
  {@const g = st.branchGone}
  <div class="banner warn">
    <div>
      <Tx text={g.by ? t("{branch} was archived by {name} {when}.") : t("{branch} was archived {when}.")}
        strong={{ branch: `“${g.label || g.name}”` }} vars={{ name: g.by, when: ago(g.time) }} />
      <span class="muted">{t("Your files and versions here are as they were.")}</span>
    </div>
    {#if onswitchmain}<button onclick={onswitchmain}>{t("Switch to main")}</button>{/if}
    {#if onnewbranch}<button onclick={onnewbranch}>{t("Go on in a new branch")}</button>{/if}
    {#if onrestorebranch}<button class="primary" onclick={onrestorebranch}>{t("Unarchive it")}</button>{/if}
  </div>
{/if}

{#if loadError}
  <div class="banner warn">
    <div>⚠ {t("R3V can't read the project right now, so what you see may be out of date: {error}", { error: loadError })}</div>
  </div>
{/if}

{#if st.inUse?.length}
  {@const first = st.inUse[0].slice(st.inUse[0].lastIndexOf("/") + 1)}
  <div class="banner info" title={st.inUse.join("\n")}>
    <div>{st.inUse.length === 1
      ? t("{file} is in use by another program (Live writing a Freeze file, say): R3V reads it once it's free.", { file: first })
      : tn(st.inUse.length - 1, "{file} and {n} more file are in use by another program (Live writing Freeze files, say): R3V reads them once they're free.",
        "{file} and {n} more files are in use by another program (Live writing Freeze files, say): R3V reads them once they're free.", { file: first })}</div>
  </div>
{/if}

{#if st.cloudFolder && !cloudOk[st.root]}
  <div class="banner warn">
    <div>
      <Tx text={t("This project is in your {cloud} folder.")} strong={{ cloud: st.cloudFolder }} />
      <span class="muted">{t("{cloud} also syncs R3V's history (the hidden .r3v folder): used from two computers it can damage it, and files kept online-only aren't really here. Best keep projects in a folder {cloud} doesn't sync: R3V and your team storage already keep them safe.", { cloud: st.cloudFolder })}</span>
    </div>
    <button onclick={() => okCloud(st.root)}>{t("I understand")}</button>
  </div>
{/if}
{#if st.unfinished}
  {@const v = st.unfinished}
  <div class="banner warn">
    <div>
      <Tx text={t("Switching to {version} didn't finish")} strong={{ version: `“${titleOf(v.message) || v.short}”` }} />
      <span class="muted">{t("— R3V was closed or a file was in use. Some files are from that version, some aren't. Put them back as they were, then try again.")}</span>
    </div>
    <button class="primary" disabled={!!busy} onclick={onrecover}>{t("Put files back")}</button>
  </div>
{/if}
{#each st.rules.suggestions as s (s.folder + s.preset)}
  <div class="banner warn rule">
    <RuleSuggestion {s} onpreset={(p) => onpreset(s, p)} />
  </div>
{/each}
{#if st.olderVersion}
  {@const v = st.olderVersion}
  <div class="banner older">
    <div>
      <Tx text={t("You're on an older version: {version}")} strong={{ version: `“${titleOf(v.message) || v.short}”` }} />
      <span class="muted">— {v.author}, {ago(v.time)}. {t("Newer versions are kept.")}</span>
    </div>
    {#if st.remoteUrl && st.changes.length}
      <button onclick={onbranchhere} disabled={!!busy}
        title={t("Commit your changes on a branch of your own, starting from this version")}>{t("New branch from here…")}</button>
      <button onclick={onlatest} disabled={!!busy}>{t("Back to latest")}</button>
      <button class="primary" onclick={oncombine} disabled={!!busy}
        title={t("Commit your changes after this version and combine them with the latest")}>{t("Preview & combine")}</button>
    {:else if st.remoteUrl}
      <button onclick={onnewbranch} disabled={!!busy}
        title={t("Continue from this version on a branch of your own")}>{t("New branch from here…")}</button>
    {:else}
      <button onclick={onkeep} disabled={!!busy}
        title={t("Continue from this version: it becomes a new, latest version")}>{t("Make this the latest…")}</button>
    {/if}
    {#if !(st.remoteUrl && st.changes.length)}
      <button class="primary" onclick={onlatest} disabled={!!busy}>{t("Back to latest")}</button>
    {/if}
  </div>
{/if}
{#if restorable && spotsDismissed !== restorable}
  <div class="banner warn">
    <div>⚠ {tn(missingSamples, "{n} sample of this project is missing.", "{n} samples of this project are missing.")}
      {tn(restorable, "R3V has a copy.", "R3V has copies of {n}.")}</div>
    <button class="ghost" onclick={() => (spotsDismissed = restorable)} disabled={!!busy}>{t("Later")}</button>
    <button class="primary" onclick={onrestore} disabled={!!busy}
      title={t("Copies them into Samples/Imported and points the sets there")}>{t("Recover from R3V")}</button>
  </div>
{/if}

{#if st.rules.error}
  <div class="banner warn">
    <div>⚠ {st.rules.error}</div>
    <button onclick={onopenrules}>{t("Open {file}", { file: ".r3v.yaml" })}</button>
  </div>
{/if}

<style>
  .banner { display: flex; align-items: center; gap: var(--sp-10); margin: var(--sp-6) var(--sp-24); padding: var(--sp-10) var(--sp-14); border-radius: var(--radius-lg); }
  .banner > div, .rule > :global(div) { flex: 1; }
  .banner.info { background: var(--info-bg); border: var(--border-width) solid var(--info-line); }
  .banner.older { background: var(--past-bg); border: var(--border-width) solid var(--past-line); }
  .banner.warn { background: var(--warn-bg); border: var(--border-width) solid var(--warn-line); color: var(--warn-text); }
</style>
