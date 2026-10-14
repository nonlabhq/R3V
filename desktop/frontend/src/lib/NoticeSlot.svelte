<script lang="ts" module>
  // A transfer that ends within a second isn't shown (no flash); once
  // shown, it stays at least 0.8 s. (Tests set these to 0.)
  export const noticeTiming = { showAfter: 1000, stay: 800 };
</script>

<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { ago, progressDetail, progressFraction, progressText, titleOf, type MemberLook, type Progress, type State, type Version } from "./api";
  import { newsOf, takenBackText } from "./teamText";
  import Avatar from "$shared/Avatar.svelte";
  import Tx from "./Tx.svelte";
  import { pressing } from "./notices.svelte";
  import type { Snippet } from "svelte";

  // The notice slot in the middle of a project's title bar (handoff
  // notices/spec.md): a place that is always there, so what comes and goes
  // in it moves nothing else. One thing at a time, most pressing first: a
  // transfer under way; else what stops the work (a switch that didn't
  // finish, an older version...); else the team's new versions (one, or a
  // digest of several); else versions taken back; else the rest of what the
  // page has to say. What else there is waits beside it as a chip (+N), its
  // list opening below (updates that come during a transfer: their own chip).
  let { st, progress, busy, looks, cancelling = false, oncancel, onqueue, onupdate, onpick, onpreview, issues = [], issue }: {
    st: State;
    progress: Progress | null;
    busy: string;
    looks?: Record<string, MemberLook | undefined>;
    cancelling?: boolean;
    oncancel?: () => void; // stop the commit under way (progress.cancellable)
    onqueue?: () => void; // the upload queue (a click on a transfer)
    onupdate: () => void; // get the team's versions
    onpick: (id: string) => void; // a version in the graph, with its details
    onpreview?: () => void; // what getting them all would change, file by file
    issues?: string[]; // what the page has to say until it's dealt with (notices.svelte.ts)
    issue?: Snippet<["lead" | "panel", string[]]>; // shows them (ProjectBanners)
  } = $props();

  let shown = $state.raw<Progress | null>(null);
  let since = 0;
  let showTimer: ReturnType<typeof setTimeout> | undefined;
  let hideTimer: ReturnType<typeof setTimeout> | undefined;
  const working = $derived(!!progress || busy === "first-share");
  $effect(() => {
    const p = progress, on = working;
    if (on) {
      clearTimeout(hideTimer);
      if (shown || since) { shown = p; return; }
      if (!showTimer) showTimer = setTimeout(() => { showTimer = undefined; since = Date.now(); shown = progress; visible = true; }, noticeTiming.showAfter);
      return;
    }
    clearTimeout(showTimer);
    showTimer = undefined;
    if (!visible) return;
    hideTimer = setTimeout(() => { visible = false; shown = null; since = 0; }, Math.max(0, noticeTiming.stay - (Date.now() - since)));
  });
  let visible = $state(false);
  $effect(() => () => { clearTimeout(showTimer); clearTimeout(hideTimer); });

  // (A download that didn't finish says so below the title: not news.)
  const news = $derived(st.remoteUrl && !st.head ? [] : newsOf(st.incoming ?? []));
  const authors = $derived([...new Map(news.map((v) => [v.authorId || v.author, v])).values()]);
  const lookOf = (v: Version) => looks?.[v.authorId];
  const names = (vs: Version[]) => {
    const n = [...new Set(vs.map((v) => v.author))];
    return n.length === 1 ? n[0] : n.length === 2 ? t("{a} and {b}", { a: n[0], b: n[1] }) : t("{a}, {b} and others", { a: n[0], b: n[1] });
  };
  // Uploads go up (↑), the team's versions come down (↓).
  const down = $derived(!!shown && ["downloading", "history", "placing"].includes(shown.stage));
  const known = $derived(!!shown && (!!shown.totalBytes || !!shown.total));
  const pct = $derived(shown && known ? Math.round(progressFraction(shown) * 100) : 0);
  const detail = $derived(shown ? progressDetail(shown) : "");
  // Files changed now would be read half-way; once it's in the history,
  // work can go on while it uploads.
  const hold = $derived(!!shown && (shown.stage === "scanning" || shown.stage === "storing"));
  const goOn = $derived(!!shown && (shown.stage === "checking" || shown.stage === "uploading"));

  // What leads, and the rest beside it.
  const lead = $derived(visible ? "transfer" : issues.length && pressing.has(issues[0]) ? "issue"
    : news.length ? "news" : st.takenBack?.length ? "takenBack" : issues.length ? "issue" : "");
  const restIssues = $derived(visible ? [] : lead === "issue" ? issues.slice(1) : issues);
  const restNews = $derived(lead === "issue" && news.length > 0);
  const restCount = $derived(restIssues.length + (restNews ? 1 : 0));

  let open = $state(false); // the digest's list
  let restOpen = $state(false); // the rest's list
  let box = $state<HTMLElement>();
  function ondown(e: PointerEvent) {
    if (box && !box.contains(e.target as Node)) { open = false; restOpen = false; }
  }
</script>

<svelte:window onpointerdown={ondown} onkeydown={(e) => { if (e.key === "Escape") { open = false; restOpen = false; } }} />

<div class="slot" bind:this={box} role="status" aria-live="polite">
  {#if lead === "transfer"}
    <!-- svelte-ignore a11y_no_static_element_interactions, a11y_no_noninteractive_tabindex -->
    <div class="transfer" class:clickable={!!onqueue} onclick={onqueue} role={onqueue ? "button" : undefined} tabindex={onqueue ? 0 : undefined}
      onkeydown={(e) => { if (onqueue && (e.key === "Enter" || e.key === " ")) { e.preventDefault(); onqueue(); } }}
      title={hold ? t("Don't change the project's files until this step is done.")
        : goOn ? t("It's in the history: you can keep working while it uploads.") : onqueue ? t("Click to see the upload queue") : ""}>
      <span class="dir" aria-hidden="true">{down ? "↓" : "↑"}</span>
      <div class="lines">
        <div class="line1">
          <span class="what">{shown ? progressText(shown, st.teamName || undefined) : t("Sharing “{name}” with the team…", { name: st.name })}</span>
          {#if hold}<span class="tag hold">{t("Don't change files yet")}</span>{:else if goOn}<span class="tag go">✓ {t("Keep working")}</span>{/if}
          <span class="nums">{known ? `${pct}%` : t("Preparing…")}{detail ? ` · ${detail}` : ""}</span>
        </div>
        <div class="bar" class:indeterminate={!known} role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={known ? pct : undefined}>
          <div style:width="{known ? pct : 30}%"></div>
        </div>
      </div>
      {#if shown?.cancellable && oncancel}
        <button class="cancel" disabled={cancelling} title={t("Stop before the team gets it: your changes stay as they are")}
          onclick={(e) => { e.stopPropagation(); oncancel(); }} onkeydown={(e) => e.stopPropagation()}>{cancelling ? t("Cancelling…") : t("Cancel")}</button>
      {/if}
    </div>
    {#if news.length}
      <span class="chip" title={tn(news.length, "{who} shared a new version — get it when this finishes", "{who} shared {n} new versions — get them when this finishes", { who: names(news) })}>
        <Avatar name={news[0].author} seed={news[0].authorId} color={lookOf(news[0])?.color} picture={lookOf(news[0])?.picture} size={18} />
        <span>+{tn(news.length, "{n} update", "{n} updates")}</span>
      </span>
    {/if}
  {:else if lead === "issue" && issue}
    {@render issue("lead", issues)}
  {:else if lead === "news" && news.length === 1}
    {@const v = news[0]}
    <div class="capsule">
      <Avatar name={v.author} seed={v.authorId} color={lookOf(v)?.color} picture={lookOf(v)?.picture} size={22} />
      <span class="say"><Tx text={t("{who} shared a new version · “{version}”")} strong={{ who: v.author }} vars={{ version: titleOf(v.message) || v.short }} /></span>
      <button class="small" onclick={() => onpick(v.id)}>{t("Preview")}</button>
      {#if !st.olderVersion}<button class="small primary" onclick={onupdate} disabled={!!busy}>{t("Get updates")}</button>{/if}
    </div>
  {:else if lead === "news"}
    <div class="capsule digest">
      <button class="open" aria-expanded={open} onclick={() => (open = !open)}>
        <span class="faces">{#each authors.slice(0, 3) as v (v.authorId || v.author)}<Avatar name={v.author} seed={v.authorId} color={lookOf(v)?.color} picture={lookOf(v)?.picture} size={22} />{/each}</span>
        <span class="say"><Tx text={t("{count} from {who}", { who: names(news) })}
          strong={{ count: tn(news.length, "{n} update", "{n} updates") }} /></span>
        <span class="caret" aria-hidden="true">▾</span>
      </button>
      {#if !st.olderVersion}<button class="small primary" onclick={onupdate} disabled={!!busy}>{t("Get all")}</button>{/if}
    </div>
    {#if open}
      <div class="list" role="menu">
        {#each news as v (v.id)}
          <div class="row">
            <Avatar name={v.author} seed={v.authorId} color={lookOf(v)?.color} picture={lookOf(v)?.picture} size={22} />
            <div class="rtext">
              <div><Tx text={v.parents.length > 1 ? t("{who} merged “{version}”") : t("{who} shared “{version}”")} strong={{ who: v.author }}
                vars={{ version: titleOf(v.message) || v.short }} /></div>
              <div class="faint small">{v.branch ? `${v.branch} · ` : ""}{ago(v.time)}</div>
            </div>
            <button class="small" onclick={() => { open = false; onpick(v.id); }}>{t("Preview")}</button>
          </div>
        {/each}
        {#if onpreview}
          <button class="all small" disabled={!!busy} onclick={() => { open = false; onpreview(); }}>{t("Preview all changes")}</button>
        {/if}
      </div>
    {/if}
  {:else if lead === "takenBack"}
    <div class="capsule">
      <span class="say">{takenBackText(st.takenBack)}</span>
      {#if !st.olderVersion}<button class="small primary" onclick={onupdate} disabled={!!busy}>{t("Get updates")}</button>{/if}
    </div>
  {/if}
  {#if restCount && issue}
    <button class="chip more" aria-expanded={restOpen} onclick={() => { restOpen = !restOpen; open = false; }}
      title={t("More to see")}>+{restCount}</button>
    {#if restOpen}
      <div class="list rest">
        {#if restNews}
          <div class="row">
            <Avatar name={news[0].author} seed={news[0].authorId} color={lookOf(news[0])?.color} picture={lookOf(news[0])?.picture} size={22} />
            <div class="rtext"><Tx text={t("{count} from {who}", { who: names(news) })} strong={{ count: tn(news.length, "{n} update", "{n} updates") }} /></div>
            {#if !st.olderVersion}<button class="small primary" onclick={() => { restOpen = false; onupdate(); }} disabled={!!busy}>{t("Get updates")}</button>{/if}
          </div>
        {/if}
        {@render issue("panel", restIssues)}
      </div>
    {/if}
  {/if}
</div>

<style>
  /* Always there, its size kept: what comes and goes moves nothing else. */
  .slot { position: relative; flex: 1; min-width: 0; height: 46px; display: flex; align-items: center; justify-content: center;
    gap: var(--sp-8); padding: 0 var(--sp-16); }
  .transfer, .capsule { display: flex; align-items: center; gap: var(--sp-10); min-width: 0; max-width: 760px; height: 42px;
    padding: 0 var(--sp-6) 0 var(--sp-12); border-radius: 12px; background: var(--hover); box-shadow: inset 0 0 0 var(--border-width) var(--line);
    animation: fade .15s ease-out; }
  .transfer { flex: 1; }
  .transfer.clickable { cursor: pointer; }
  @keyframes fade { from { opacity: 0; } }
  @media (prefers-reduced-motion: reduce) { .transfer, .capsule { animation: none; } }
  .dir { color: var(--accent); font-weight: var(--fw-semibold); }
  .lines { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: var(--sp-4); }
  .line1 { display: flex; gap: var(--sp-10); font-size: var(--fs-sm); min-width: 0; }
  .what { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tag { flex: none; }
  .tag.hold { color: var(--warn-text); }
  .tag.go { color: var(--accent); }
  .nums { flex: none; color: var(--muted); font-variant-numeric: tabular-nums; }
  .bar { position: relative; height: 4px; border-radius: 2px; background: var(--line); overflow: hidden; }
  .bar > div { height: 100%; background: var(--accent); transition: width .2s; }
  .bar.indeterminate > div { animation: slide 1.2s ease-in-out infinite; }
  @keyframes slide { from { transform: translateX(-100%); } to { transform: translateX(340%); } }
  .cancel { flex: none; }
  .chip { flex: none; display: inline-flex; align-items: center; gap: var(--sp-6); height: 32px; padding: 0 var(--sp-10) 0 var(--sp-6);
    border-radius: var(--radius-pill); background: var(--hover); box-shadow: inset 0 0 0 var(--border-width) var(--line); font-size: var(--fs-sm); }
  .say { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: var(--fs-sm); }
  .capsule button { flex: none; }
  .digest .open { display: flex; align-items: center; gap: var(--sp-8); min-width: 0; background: none; border: none; padding: 0; color: inherit; cursor: pointer; }
  .faces { display: inline-flex; }
  .faces > :global(*:not(:first-child)) { margin-left: -6px; }
  .caret { color: var(--faint); }
  .list { position: absolute; top: calc(100% + var(--sp-4)); left: 50%; transform: translateX(-50%); width: 380px; z-index: var(--z-menu);
    padding: var(--sp-6); border-radius: var(--radius-lg); background: var(--surface-menu); backdrop-filter: var(--surface-filter);
    box-shadow: var(--shadow-pop); border: var(--border-width) solid var(--line); }
  .row { display: flex; align-items: center; gap: var(--sp-10); padding: var(--sp-8); border-radius: var(--radius); }
  .row:hover { background: var(--hover); }
  .rtext { flex: 1; min-width: 0; font-size: var(--fs-sm); }
  .rtext > div { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .small { font-size: var(--fs-sm); }
  .more { cursor: pointer; border: none; color: inherit; padding: 0 var(--sp-10); font-variant-numeric: tabular-nums; }
  .rest { width: 440px; max-height: 60vh; overflow: auto; }
  .all { width: 100%; margin-top: var(--sp-4); }
</style>
