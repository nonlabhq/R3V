<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { ago, titleOf, type State, type RuleSuggestion as Suggestion } from "./api";
  import Tx from "./Tx.svelte";
  import RuleSuggestion from "./RuleSuggestion.svelte";
  import { putAside, understandCloud } from "./notices.svelte";

  // What a project's page has to say until it's dealt with (the keys, most
  // pressing first: notices.svelte.ts), in the title bar's notice slot.
  // "lead": the first as a capsule on one line, its main button beside it; a
  // click on the words opens the whole of it with the other buttons.
  // "panel": each in full, for the slot's list of the rest. Each says what
  // to do; the page does it.
  let { mode, keys, st, busy, restorable, missingSamples, onshare, onrecover, onpreset, onbranchhere, onlatest,
    oncombine, onnewbranch, onkeep, onupdate, onrestore, onopenrules, loadError = "",
    onrestorebranch, onswitchmain, onlockson, onlocksnotnow }: {
    mode: "lead" | "panel";
    keys: string[];
    st: State;
    busy: string;
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

  const team = $derived(st.teamName || t("the team"));
  const ruleOf = (k: string) => st.rules.suggestions.find((s) => k === `rule:${s.folder}\n${s.preset}`);
  const tone = (k: string) => (k === "older" ? "older" : ["notShared", "unshared", "inUse", "locks"].includes(k) ? "info" : "warn");

  let open = $state(false); // the lead's whole text
  let box = $state<HTMLElement>();
  $effect(() => { void keys[0]; open = false; });
</script>

<svelte:window onpointerdown={(e) => { if (open && box && !box.contains(e.target as Node)) open = false; }}
  onkeydown={(e) => { if (open && e.key === "Escape") open = false; }} />

{#snippet text(k: string)}
  {#if k === "notDownloaded"}
    {t("The download didn't finish (the connection was lost or R3V closed): get the files to finish it.")}
  {:else if k === "notShared"}
    {t("Not shared with {team} yet. Look through the files and ignore the folders or files you don't need (right-click › Ignore), then commit a first version to share it.", { team })}
  {:else if k === "unshared"}
    {t("Not shared with {team} yet: its versions are on this computer only.", { team })}
  {:else if k === "locks"}
    {t("Files like these can't be merged: when two people change the same one, one of them loses their work. With file locking, {team} sees who is working on which file, and a change to a file someone else holds waits until they're done.", { team })}
  {:else if k === "branchGone" && st.branchGone}
    {@const g = st.branchGone}
    <Tx text={g.by ? t("{branch} was archived by {name} {when}.") : t("{branch} was archived {when}.")}
      strong={{ branch: `“${g.label || g.name}”` }} vars={{ name: g.by, when: ago(g.time) }} />
    <span class="muted">{t("Your files and versions here are as they were.")}</span>
  {:else if k === "loadError"}
    ⚠ {t("R3V can't read the project right now, so what you see may be out of date: {error}", { error: loadError })}
  {:else if k === "inUse" && st.inUse?.length}
    {@const first = st.inUse[0].slice(st.inUse[0].lastIndexOf("/") + 1)}
    <!-- (Live's own example for a Live project: a Freeze file it's writing) -->
    {#if st.tool === "Ableton Live"}
      {st.inUse.length === 1
        ? t("{file} is in use by another program (Live writing a Freeze file, say): R3V reads it once it's free.", { file: first })
        : tn(st.inUse.length - 1, "{file} and {n} more file are in use by another program (Live writing Freeze files, say): R3V reads them once they're free.",
          "{file} and {n} more files are in use by another program (Live writing Freeze files, say): R3V reads them once they're free.", { file: first })}
    {:else}
      {st.inUse.length === 1
        ? t("{file} is in use by another program (still writing it, say): R3V reads it once it's free.", { file: first })
        : tn(st.inUse.length - 1, "{file} and {n} more file are in use by another program (still writing them, say): R3V reads them once they're free.",
          "{file} and {n} more files are in use by another program (still writing them, say): R3V reads them once they're free.", { file: first })}
    {/if}
  {:else if k === "cloud" && st.cloudFolder}
    <Tx text={t("This project is in your {cloud} folder.")} strong={{ cloud: st.cloudFolder }} />
    <span class="muted">{t("{cloud} also syncs R3V's history (the hidden .r3v folder): used from two computers it can damage it, and files kept online-only aren't really here. Best keep projects in a folder {cloud} doesn't sync: R3V and your team storage already keep them safe.", { cloud: st.cloudFolder })}</span>
  {:else if k === "unfinished" && st.unfinished}
    {@const v = st.unfinished}
    <Tx text={t("Switching to {version} didn't finish")} strong={{ version: `“${titleOf(v.message) || v.short}”` }} />
    <span class="muted">{t("— R3V was closed or a file was in use. Some files are from that version, some aren't. Put them back as they were, then try again.")}</span>
  {:else if k === "older" && st.olderVersion}
    {@const v = st.olderVersion}
    <Tx text={t("You're on an older version: {version}")} strong={{ version: `“${titleOf(v.message) || v.short}”` }} />
    <span class="muted">— {v.author}, {ago(v.time)}. {t("Newer versions are kept.")}</span>
  {:else if k === "samples"}
    ⚠ {tn(missingSamples, "{n} sample of this project is missing.", "{n} samples of this project are missing.")}
    {tn(restorable, "R3V has a copy.", "R3V has copies of {n}.")}
  {:else if k === "rulesError"}
    ⚠ {st.rules.error}
  {:else if k.startsWith("rule:")}
    {@const s = ruleOf(k)}
    {#if s}<RuleSuggestion {s} onpreset={(p) => onpreset(s, p)} part="text" />{/if}
  {/if}
{/snippet}

<!-- the button that does what it asks (beside the lead's words) -->
{#snippet main(k: string)}
  {#if k === "notDownloaded"}
    <button class="primary" onclick={onupdate} disabled={!!busy}>{t("Finish downloading")}</button>
  {:else if k === "unshared"}
    <button class="primary" onclick={onshare} disabled={!!busy}>{t("Share now")}</button>
  {:else if k === "locks" && onlockson}
    <button class="primary" onclick={onlockson}>{t("Turn on file locking")}</button>
  {:else if k === "branchGone" && onrestorebranch}
    <button class="primary" onclick={onrestorebranch}>{t("Unarchive it")}</button>
  {:else if k === "cloud"}
    <button onclick={() => understandCloud(st.root)}>{t("I understand")}</button>
  {:else if k === "unfinished"}
    <button class="primary" disabled={!!busy} onclick={onrecover}>{t("Put files back")}</button>
  {:else if k === "older"}
    {#if st.remoteUrl && st.changes.length}
      <button class="primary" onclick={oncombine} disabled={!!busy}
        title={t("Commit your changes after this version and combine them with the latest")}>{t("Preview & combine")}</button>
    {:else}
      <button class="primary" onclick={onlatest} disabled={!!busy}>{t("Back to latest")}</button>
    {/if}
  {:else if k === "samples"}
    <button class="primary" onclick={onrestore} disabled={!!busy}
      title={t("Copies them into Samples/Imported and points the sets there")}>{t("Recover from R3V")}</button>
  {:else if k === "rulesError"}
    <button onclick={onopenrules}>{t("Open {file}", { file: ".r3v.yaml" })}</button>
  {:else if k.startsWith("rule:")}
    {@const s = ruleOf(k)}
    {#if s}<RuleSuggestion {s} onpreset={(p) => onpreset(s, p)} part="main" />{/if}
  {/if}
{/snippet}

<!-- the other choices, with the whole text -->
{#snippet more(k: string)}
  {#if k === "locks" && onlocksnotnow}
    <button onclick={onlocksnotnow}>{t("Not now")}</button>
  {:else if k === "branchGone"}
    {#if onswitchmain}<button onclick={onswitchmain}>{t("Switch to main")}</button>{/if}
    <button onclick={onnewbranch}>{t("Go on in a new branch")}</button>
  {:else if k === "older"}
    {#if st.remoteUrl && st.changes.length}
      <button onclick={onbranchhere} disabled={!!busy}
        title={t("Commit your changes on a branch of your own, starting from this version")}>{t("New branch from here…")}</button>
      <button onclick={onlatest} disabled={!!busy}>{t("Back to latest")}</button>
    {:else if st.remoteUrl}
      <button onclick={onnewbranch} disabled={!!busy}
        title={t("Continue from this version on a branch of your own")}>{t("New branch from here…")}</button>
    {:else}
      <button onclick={onkeep} disabled={!!busy}
        title={t("Continue from this version: it becomes a new, latest version")}>{t("Make this the latest…")}</button>
    {/if}
  {:else if k === "samples"}
    <button class="ghost" onclick={() => (putAside.samples[st.root] = restorable)} disabled={!!busy}>{t("Later")}</button>
  {:else if k.startsWith("rule:")}
    {@const s = ruleOf(k)}
    {#if s}<RuleSuggestion {s} onpreset={(p) => onpreset(s, p)} part="more" />{/if}
  {/if}
{/snippet}

{#if mode === "lead" && keys.length}
  {@const k = keys[0]}
  <div class="lead" bind:this={box}>
    <div class="capsule {tone(k)}">
      <button class="say" aria-expanded={open} onclick={() => (open = !open)}><span class="words">{@render text(k)}</span><span class="caret" aria-hidden="true">▾</span></button>
      {@render main(k)}
    </div>
    {#if open}
      <div class="pop {tone(k)}">
        <div class="whole">{@render text(k)}</div>
        <div class="buttons">{@render more(k)}</div>
      </div>
    {/if}
  </div>
{:else if mode === "panel"}
  {#each keys as k (k)}
    <div class="card {tone(k)}">
      <div class="whole">{@render text(k)}</div>
      <div class="buttons">{@render more(k)}{@render main(k)}</div>
    </div>
  {/each}
{/if}

<style>
  .lead { display: flex; justify-content: center; min-width: 0; max-width: 760px; flex: 0 1 auto; }
  .capsule { display: flex; align-items: center; gap: var(--sp-8); min-width: 0; height: 42px; padding: 0 var(--sp-6) 0 var(--sp-4);
    border-radius: 12px; animation: fade .15s ease-out; }
  @keyframes fade { from { opacity: 0; } }
  @media (prefers-reduced-motion: reduce) { .capsule { animation: none; } }
  .capsule > :global(button) { flex: none; font-size: var(--fs-sm); }
  .capsule > .say { flex: 0 1 auto; display: flex; align-items: center; gap: var(--sp-6); min-width: 0; padding: 0 var(--sp-8);
    background: none; border: none; color: inherit; cursor: pointer; text-align: left; }
  .words { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: var(--fs-sm); }
  .words :global(div) { display: inline; }
  .caret { flex: none; color: var(--faint); }
  .pop { position: absolute; top: calc(100% + var(--sp-4)); left: 50%; transform: translateX(-50%); width: min(480px, 90vw); z-index: var(--z-menu);
    padding: var(--sp-12) var(--sp-14); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop); font-size: var(--fs-sm); }
  .card { padding: var(--sp-10) var(--sp-12); border-radius: var(--radius); font-size: var(--fs-sm); }
  .card + .card { margin-top: var(--sp-6); }
  .buttons { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: var(--sp-8); }
  .buttons:not(:empty) { margin-top: var(--sp-8); }
  .info { background: var(--info-bg); box-shadow: inset 0 0 0 var(--border-width) var(--info-line); }
  .older { background: var(--past-bg); box-shadow: inset 0 0 0 var(--border-width) var(--past-line); }
  .warn { background: var(--warn-bg); box-shadow: inset 0 0 0 var(--border-width) var(--warn-line); color: var(--warn-text); }
  .pop.info, .pop.older, .pop.warn { box-shadow: var(--shadow-pop), inset 0 0 0 var(--border-width) var(--line); }
</style>
