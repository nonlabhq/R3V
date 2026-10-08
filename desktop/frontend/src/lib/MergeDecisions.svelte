<script lang="ts" module>
  // What brought up the decisions: merging a branch or a version, the team's
  // versions coming in (update, commit), or undoing a version.
  export type MergeKind = "merge" | "update" | "undo";
</script>

<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { ago, type Conflict, type MemberLook, type Version } from "./api";
  import type { Combined } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";
  import { branchLabel, branchLane, type BranchLook } from "./branches";
  import Avatar from "./Avatar.svelte";
  import FileIcon from "./FileIcon.svelte";
  import Modal from "./Modal.svelte";
  import Tx from "./Tx.svelte";
  import MergeSetPreview from "./MergeSetPreview.svelte";

  // Things both sides changed, decided one at a time over the whole app:
  // keep yours, keep theirs, or (where it can be) both. Nothing is changed
  // until the last decision; onresolve gets them all, by Conflict.key.
  let { conflicts, ours = null, theirs = null, combined = [], kind = "update", root, project = "",
    me = "", ourBranch = "", theirBranch = "", version = "", branches = [], looks, onresolve, onclose }: {
    conflicts: Conflict[];
    ours?: Version | null;    // your side (no id: the files as they are now)
    theirs?: Version | null;  // the other side
    combined?: Combined[];    // what came in on its own
    kind?: MergeKind;
    root: string;
    project?: string;
    me?: string;              // your name
    ourBranch?: string;       // branch keys ("" not known)
    theirBranch?: string;
    version?: string;         // undo: the version being undone, merge: the version merged when not a branch
    branches?: BranchLook[];
    looks?: Record<string, MemberLook | undefined>;
    onresolve: (resolutions: Record<string, string>) => void;
    onclose: () => void;
  } = $props();

  let at = $state(0);
  let panel = $state<HTMLDivElement>();
  let choices = $state<Record<string, string>>({});
  let confirming = $state(false);
  let c = $derived(conflicts[at]);
  let last = $derived(at === conflicts.length - 1);

  // (results from before the sides were sent: told apart as they were)
  const kindOf = (x: Conflict) => x.kind || (x.unit === x.file ? "file" : "set");
  const nameOf = (x: Conflict) => x.name || (x.unit === x.file ? x.file.slice(x.file.lastIndexOf("/") + 1) : x.unit);
  const oursDid = (x: Conflict) => x.ours || "changed";
  const theirsDid = (x: Conflict) => x.theirs || "changed";
  // Both kept: only where it differs from keeping one side.
  const canBoth = (x: Conflict) => x.canKeepBoth && oursDid(x) === "changed" && theirsDid(x) === "changed";
  const options = (x: Conflict) => (canBoth(x) ? ["ours", "theirs", "both"] : ["ours", "theirs"]);

  // The other person: the other side's author, unless that is you.
  let other = $derived(theirs && theirs.author && !(theirs.authorId && theirs.authorId === ours?.authorId)
    && !(!theirs.authorId && theirs.author === (ours?.author || me)) ? theirs.author : "");
  let theirKey = $derived(kind === "merge" ? theirBranch : ourBranch); // (merging a version on no branch: "")
  let sameBranch = $derived(kind !== "merge" || theirBranch === ourBranch);
  const bl = (key: string) => branchLabel(branches, key);
  const lane = (key: string) => (key ? `var(--lane-${branchLane(branches, key)})` : "var(--faint)");
  let ourColor = $derived(lane(ourBranch));
  let theirColor = $derived(kind === "undo" ? "var(--faint)" : lane(theirKey));

  // ---- words ----
  let title = $derived.by(() => {
    if (!c) return "";
    const k = kindOf(c);
    const order = k === "set" && c.key.endsWith("#order");
    const v = { who: other }; // ({track} and {file} are filled in below, shown apart)
    if (kind === "undo") {
      return k === "track" ? t("The {track} track changed again after that version", v)
        : k === "file" ? t("{file} changed again after that version", v)
        : order ? t("The track order in {file} changed again after that version", v)
        : t("That part of {file} changed again after that version", v);
    }
    if (other) {
      return k === "track" ? t("You and {who} both changed the {track} track", v)
        : k === "file" ? t("You and {who} both changed {file}", v)
        : order ? t("You and {who} both changed the track order in {file}", v)
        : t("You and {who} both changed the same part of {file}", v);
    }
    return k === "track" ? t("Both sides changed the {track} track", v)
      : k === "file" ? t("Both sides changed {file}", v)
      : order ? t("Both sides changed the track order in {file}", v)
      : t("Both sides changed the same part of {file}", v);
  });
  let titleVars = $derived<Record<string, string>>(c ? { track: nameOf(c), file: c.file.slice(c.file.lastIndexOf("/") + 1) } : {});
  let fullTitle = $derived(Object.entries(titleVars).reduce((s, [k, x]) => s.replaceAll(`{${k}}`, x), title));

  const MOST = 4;
  let combinedText = $derived.by(() => {
    const words = combined.slice(0, MOST).map((x) => {
      const v = { name: x.name || x.file.slice(x.file.lastIndexOf("/") + 1) };
      return x.what === "added" ? t("{name} added", v) : x.what === "removed" ? t("{name} removed", v) : t("{name} changed", v);
    });
    if (combined.length > MOST) words.push(tn(combined.length - MOST, "and {n} more", "and {n} more"));
    return words.length ? t("Everything else was combined on its own: {list}.", { list: words.join(", ") })
      : t("Everything else was combined on its own.");
  });

  let yoursLabel = $derived(kind === "undo" ? t("Keep it as it is now")
    : other ? t("Keep yours")
    : !sameBranch ? t("Keep {branch}'s", { branch: bl(ourBranch) })
    : t("Keep this computer's"));
  let theirsLabel = $derived(kind === "undo" ? t("Take it back anyway")
    : other ? t("Keep {who}'s", { who: other })
    : !sameBranch ? t("Keep {branch}'s", { branch: theirBranch ? bl(theirBranch) : version })
    : t("Keep the team's"));

  // "on main · Bass EQ · 2 h ago"
  function where(v: Version | null, branch: string): string {
    if (!v) return branch ? t("on {branch}", { branch: bl(branch) }) : "";
    if (!v.id) return t("Your changes, not committed yet");
    return [branch ? t("on {branch}", { branch: bl(branch) }) : "", v.message || v.short, ago(v.time)].filter(Boolean).join(" · ");
  }
  let yoursWhere = $derived(where(ours, ourBranch));
  let theirsWhere = $derived(kind === "undo"
    ? (version ? t("As before “{version}”", { version }) : "")
    : where(theirs, theirKey));

  function did(x: Conflict, side: "ours" | "theirs"): string {
    const del = (side === "ours" ? oursDid(x) : theirsDid(x)) === "deleted";
    if (kind === "undo") {
      return side === "ours" ? (del ? t("Deleted since") : t("Changed since")) : (del ? t("Not there before") : t("As it was before"));
    }
    const who = side === "ours" ? (other ? "you" : "") : other;
    if (who === "you") return del ? t("Deleted by you") : t("Changed by you");
    if (who) return del ? t("Deleted by {who}", { who }) : t("Changed by {who}", { who });
    return del ? t("Deleted in this version") : t("Changed in this version");
  }

  let bothText = $derived.by(() => {
    if (!c) return "";
    const name = nameOf(c);
    if (kindOf(c) === "track") {
      const copy = `${name} [theirs]`; // (internal/merge names the copy so)
      return other ? t("Keeps yours and adds {who}'s as another track, “{copy}”", { who: other, copy })
        : t("Keeps this one and adds the other as another track, “{copy}”", { copy });
    }
    const dot = name.lastIndexOf(".");
    const copy = dot > 0 ? `${name.slice(0, dot)} (theirs)${name.slice(dot)}` : `${name} (theirs)`;
    return other ? t("Keeps yours and adds {who}'s next to it, as “{copy}”", { who: other, copy })
      : t("Keeps this one and adds the other next to it, as “{copy}”", { copy });
  });

  let finish = $derived(kind === "merge" ? t("Merge") : kind === "undo" ? t("Undo") : t("Combine"));
  let cancelLabel = $derived(kind === "undo" ? t("Cancel") : t("Cancel merge"));

  // ---- moving ----
  function pick(choice: string) {
    choices[c.key] = choice;
  }
  function next() {
    if (!c || !choices[c.key]) return;
    if (last) onresolve({ ...choices });
    else at++;
  }
  function back() {
    if (at > 0) at--;
  }
  // The choice made here, for the rest still open (where it can be).
  function sameForRest() {
    const choice = choices[c.key];
    for (const x of conflicts.slice(at + 1)) {
      if (!choices[x.key]) choices[x.key] = options(x).includes(choice) ? choice : "ours";
    }
    at = conflicts.length - 1;
  }
  function cancel() {
    if (Object.keys(choices).length) confirming = true;
    else onclose();
  }
  function onkeydown(e: KeyboardEvent) {
    if (confirming || e.defaultPrevented) return; // (the question on top has the keys)
    if (e.key === "Escape") {
      e.preventDefault();
      cancel();
    } else if (e.key === "Enter" && !(e.target instanceof HTMLButtonElement) && !(e.target instanceof HTMLInputElement)) {
      e.preventDefault();
      next();
    } else if ((e.key === "ArrowLeft" || e.key === "ArrowRight") && e.target instanceof HTMLElement && e.target.getAttribute("role") === "radio") {
      e.preventDefault();
      const opts = options(c), i = opts.indexOf(choices[c.key] ?? "");
      const to = opts[(Math.max(i, 0) + (e.key === "ArrowRight" ? (i < 0 ? 0 : 1) : opts.length - 1)) % opts.length];
      pick(to);
      panel?.querySelector<HTMLElement>(`[data-choice="${to}"]`)?.focus();
    }
  }
  const radioKey = (e: KeyboardEvent, choice: string) => {
    if (e.key === " ") { e.preventDefault(); pick(choice); }
  };
  // Each decision starts with its cards in reach of the keyboard.
  $effect(() => {
    at;
    panel?.querySelector<HTMLElement>('[role="radio"][tabindex="0"]')?.focus();
  });
  const audio = /\.(wav|aiff?|mp3|flac|ogg|m4a)$/i;
  const fileKind = (p: string) => (/\.als$/i.test(p) ? "set" : audio.test(p) ? "audio" : "other");
</script>

<svelte:window {onkeydown} />

{#snippet branchDot(key: string, color: string)}
  <span class="br" style:--c={color}><i></i>{bl(key)}</span>
{/snippet}

{#snippet avatar(v: Version | null, name: string, size: number)}
  <Avatar {name} seed={v?.authorId ?? ""} color={looks?.[v?.authorId ?? ""]?.color ?? ""} picture={looks?.[v?.authorId ?? ""]?.picture ?? ""} {size} />
{/snippet}

{#snippet radio(on: boolean)}
  <span class="radio" class:on aria-hidden="true"><i></i></span>
{/snippet}

{#snippet card(x: Conflict, which: "ours" | "theirs", label: string, sub: string, v: Version | null, who: string)}
  {@const on = choices[x.key] === which}
  <div class="card" class:on role="radio" aria-checked={on} tabindex={on || (!choices[x.key] && which === "ours") ? 0 : -1}
    data-choice={which} onclick={() => pick(which)} onkeydown={(e) => radioKey(e, which)}>
    <div class="top">
      {@render radio(on)}
      <span class="label">
        <span class="name">{label}</span>
        {#if sub}<span class="faint sub">{sub}</span>{/if}
      </span>
      {#if kind !== "undo" && who}{@render avatar(v, who, 26)}{/if}
    </div>
    {@render side(x, which)}
  </div>
{/snippet}

{#snippet side(x: Conflict, which: "ours" | "theirs")}
  {@const v = which === "ours" ? ours : theirs}
  {@const color = which === "ours" ? ourColor : theirColor}
  {#if kindOf(x) === "track" && (v || which === "ours")}
    <MergeSetPreview {root} file={x.file} version={v?.id ?? ""} track={x.track} {color} name={nameOf(x)} />
  {:else}
    <div class="summary">
      <FileIcon path={x.file} kind={fileKind(x.file)} faint={(which === "ours" ? oursDid(x) : theirsDid(x)) === "deleted"} />
      <span class="what">
        <span class="file" title={x.file}>{kindOf(x) === "set" ? x.unit : x.file}</span>
        <span class="faint">{kindOf(x) === "set" ? x.file + " · " : ""}{did(x, which)}</span>
      </span>
    </div>
  {/if}
{/snippet}

<div class="scrim" role="presentation">
  <div class="panel surface-dialog" role="dialog" aria-modal="true" aria-label={fullTitle} bind:this={panel}>
    {#if c}
      <header>
        <div class="heading">
          <div class="context faint">
            {#if kind === "merge"}
              {@const parts = t("Merging {from} into {into}").split(/(\{from\}|\{into\})/)}
              {#each parts as p}{#if p === "{from}"}{#if theirBranch}{@render branchDot(theirBranch, theirColor)}{:else}<span class="ver">{version}</span>{/if}{:else if p === "{into}"}{@render branchDot(ourBranch, ourColor)}{:else}{p}{/if}{/each}
            {:else if kind === "undo"}
              {t("Undoing “{version}”", { version })}
            {:else}
              {@const parts = t("Bringing the team's versions into {branch}").split(/(\{branch\})/)}
              {#each parts as p}{#if p === "{branch}"}{@render branchDot(ourBranch, ourColor)}{:else}{p}{/if}{/each}
            {/if}
            {#if project}<span> · {project}</span>{/if}
          </div>
          <h2>
            {#if kind !== "undo"}
              <span class="pair">
                {@render avatar(ours, ours?.author || me, 30)}
                {#if other}{@render avatar(theirs, other, 30)}{/if}
              </span>
            {/if}
            <span><Tx text={title} em={titleVars} vars={{ who: other }} /></span>
          </h2>
        </div>
        <div class="progress">
          <span class="faint">{tn(conflicts.length, "{i} of {n} decision", "{i} of {n} decisions", { i: at + 1 })}</span>
          <span class="bar" aria-hidden="true">
            {#each conflicts as x, i (x.key)}<i class:done={!!choices[x.key]} class:now={i === at}></i>{/each}
          </span>
        </div>
      </header>

      <p class="explain">
        {#if kind === "undo"}
          {t("A later version changed this too. Keep it as it is now, or take it back to how it was before.")}
        {:else}
          {combinedText}
          {t("Pick which one to keep: nothing is lost, the other stays in the history.")}
          {#if other}{t("Not sure? Ask {who} before you decide.", { who: other })}{/if}
        {/if}
      </p>

      <div class="cards" role="radiogroup" aria-label={nameOf(c)}>
        {@render card(c, "ours", yoursLabel, yoursWhere, ours, ours?.author || me)}
        {@render card(c, "theirs", theirsLabel, theirsWhere, theirs, other)}
      </div>
      {#if canBoth(c)}
        {@const on = choices[c.key] === "both"}
        <div class="card both" class:on role="radio" aria-checked={on} tabindex={on ? 0 : -1} data-choice="both"
          onclick={() => pick("both")} onkeydown={(e) => radioKey(e, "both")}>
          <div class="top">
            {@render radio(on)}
            <span class="label">
              <span class="name">{t("Keep both")}</span>
              <span class="faint sub">{bothText}</span>
            </span>
          </div>
        </div>
      {/if}

      <footer>
        <button onclick={cancel}>{cancelLabel}</button>
        {#if conflicts.length > 1 && !last && choices[c.key]}
          <button class="ghost" onclick={sameForRest} title={t("Decide the rest the same way, where it can be")}>{t("Same for the rest")}</button>
        {/if}
        <span class="grow"></span>
        {#if at > 0}<button onclick={back}>← {t("Back")}</button>{/if}
        <button class="primary" disabled={!choices[c.key]} onclick={next}>{last ? finish : t("Next decision") + " →"}</button>
      </footer>
    {/if}
  </div>
</div>

{#if confirming}
  <Modal title={kind === "undo" ? t("Stop undoing?") : t("Cancel the merge?")} onclose={() => (confirming = false)}>
    <p>{t("The decisions you made are forgotten and nothing changes.")}</p>
    {#snippet footer()}
      <button onclick={() => (confirming = false)}>{t("Keep deciding")}</button>
      <button class="primary" onclick={onclose}>{cancelLabel}</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .scrim {
    position: fixed; inset: 0; z-index: var(--z-dialog); display: flex; align-items: center; justify-content: center;
    background: var(--scrim-strong); backdrop-filter: var(--scrim-blur); padding: var(--sp-24);
  }
  .panel {
    width: 1080px; max-width: 100%; max-height: 100%; overflow: auto; outline: none;
    border: var(--border-width) solid var(--line); border-radius: var(--radius-card); box-shadow: var(--shadow-dialog);
    padding: var(--sp-24) var(--sp-28); display: flex; flex-direction: column; gap: var(--sp-18);
  }
  header { display: flex; align-items: flex-start; gap: var(--sp-20); }
  .heading { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: var(--sp-6); }
  .context { font-size: var(--fs-sm); }
  .br { color: var(--c); white-space: nowrap; }
  .br i { display: inline-block; width: 9px; height: 9px; border-radius: 50%; background: var(--c); margin: 0 var(--sp-4) 0 var(--sp-2); }
  .ver { color: var(--text); }
  h2 { margin: 0; display: flex; align-items: center; gap: var(--sp-12); font-size: var(--fs-2xl); font-weight: var(--fw-bold); line-height: 1.25; }
  h2 :global(em) { font-style: normal; color: var(--accent); }
  .pair { display: inline-flex; flex: none; }
  .pair > :global(.av + .av) { margin-left: -8px; box-shadow: 0 0 0 2px var(--panel-2); }
  .progress { flex: none; display: flex; flex-direction: column; align-items: flex-end; gap: var(--sp-6); font-size: var(--fs-sm); padding-top: var(--sp-10); }
  .bar { display: flex; gap: var(--sp-4); max-width: 260px; }
  .bar i { width: 22px; min-width: 4px; flex: 0 1 auto; height: 6px; border-radius: var(--radius-pill); background: var(--line-strong); }
  .bar i.done { background: var(--accent); }
  .bar i.now { width: 36px; background: var(--accent); }
  .bar i.now:not(.done) { background: color-mix(in srgb, var(--accent) 55%, var(--line-strong)); }
  .explain { margin: 0; color: var(--muted); line-height: 1.5; }
  .cards { display: flex; gap: var(--sp-16); }
  .card {
    flex: 1; min-width: 0; display: flex; flex-direction: column; gap: var(--sp-12); cursor: pointer;
    padding: var(--sp-18); border-radius: var(--radius-card); background: var(--panel);
    box-shadow: 0 0 0 var(--border-width) var(--line); outline: none;
  }
  .card:hover { box-shadow: 0 0 0 var(--border-width) var(--line-strong); }
  .card:focus-visible { box-shadow: 0 0 0 2px var(--line-strong); }
  .card.on { background: var(--accent-soft); box-shadow: 0 0 0 2px var(--accent); }
  .card.both { flex: none; padding: var(--sp-12) var(--sp-14); border-radius: var(--radius-lg); }
  .top { display: flex; align-items: center; gap: var(--sp-10); min-width: 0; }
  .label { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .name { font-size: var(--fs-lg); font-weight: var(--fw-bold); }
  .both .name { font-size: var(--fs-base); font-weight: var(--fw-semibold); }
  .sub { font-size: var(--fs-sm); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .radio { flex: none; width: 20px; height: 20px; border-radius: 50%; border: 2px solid var(--faint); box-sizing: border-box;
    display: inline-flex; align-items: center; justify-content: center; }
  .radio.on { border-color: var(--accent); }
  .radio.on i { width: 10px; height: 10px; border-radius: 50%; background: var(--accent); }
  .summary { display: flex; align-items: center; gap: var(--sp-10); padding: var(--sp-10) var(--sp-12); border-radius: var(--radius-lg); background: var(--line-soft); }
  .what { display: flex; flex-direction: column; min-width: 0; }
  .file { font-weight: var(--fw-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  footer { display: flex; align-items: center; gap: var(--sp-8); }
  footer .primary { padding-left: var(--sp-18); padding-right: var(--sp-18); }
  .grow { flex: 1; }
</style>
