<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, detailsOf, errorText, titleOf, type ProjectFile, type Version } from "./api";
  import AgentBadge from "$shared/AgentBadge.svelte";
  import StepsDialog from "./StepsDialog.svelte";
  import type { Session } from "./api";
  import VersionChanges from "$shared/VersionChanges.svelte";
  import { changeSides } from "$shared/viewers/source";
  import { viewerFor } from "./viewers";
  import type { Snippet } from "svelte";

  // A version in the Overview: what it is (header, with what can be done
  // with it), the files it changed, and the picked one against the version
  // before it, shown by its kind's viewer.
  let { root, v, branch, actions, marks }: {
    root: string; v: Version;
    branch: string;   // the branch it is on, when known
    actions?: Snippet; // buttons for this version (Merge, Go to, …)
    marks?: Snippet;   // under the title: its milestones
  } = $props();

  let files = $state<ProjectFile[] | null>(null);
  let error = $state("");
  let picked = $state("");
  // By value: the project read again (on focus, say) brings the same
  // version as a new object, and the props (getters) follow the app's lists
  // read again too; neither must read the files again (the list and the
  // diff would blink, the picked file go back to the first).
  let vid = $derived(v.id);
  let at = $derived(root);
  let parent = $derived(v.parents[0] ?? "");
  // The steps it was made of (an agent's moves, kept on this computer):
  // there to look back at, not in the way.
  let steps = $state<Session[]>([]);
  let showSteps = $state(false);
  const stepCount = $derived(steps.reduce((n, s) => n + s.steps, 0));
  $effect(() => {
    const id = vid;
    steps = [];
    api.VersionSteps(at, id).then((s) => { if (id === vid) steps = s ?? []; }).catch(() => {});
  });
  $effect(() => {
    const id = vid;
    files = null;
    error = "";
    picked = "";
    api.VersionFiles(at, id).then((f) => {
      if (id !== vid) return;
      files = f ?? [];
      picked = files[0]?.path ?? "";
    }).catch((e) => {
      if (id === vid) error = errorText(e); // (not one for a version picked before)
    });
  });
  let current = $derived(files?.find((f) => f.path === picked));

  // This version (a) against the one before it (b).
  const sides = (f: ProjectFile) => changeSides(f, vid, parent, { now: t("In this version"), before: t("Before") });
</script>

{#if showSteps}
  <StepsDialog root={at} title={t("How “{version}” was made", { version: titleOf(v.message) || v.short })} sessions={steps}
    onclose={() => (showSteps = false)} />
{/if}

<div class="version">
  <header>
    <div class="title">
      <h2>{titleOf(v.message) || t("(no description)")}</h2>
      <div class="meta">
        {#if v.actor}<AgentBadge tool={v.actor.tool} model={v.actor.model} />{/if}
        {v.author} · {new Date(v.time).toLocaleString()}{#if branch} · {branch}{/if} · <span class="mono">{v.short}</span>
        {#if v.parents.length > 1} · {t("merge")}{/if}
        {#if stepCount}<button class="steps-btn" onclick={() => (showSteps = true)} title={t("The agent's prompts and steps that made it")}>
          <span class="dot" aria-hidden="true"></span>{tn(stepCount, "made in {n} step", "made in {n} steps")}</button>{/if}
      </div>
      {#if detailsOf(v.message)}<p class="details">{detailsOf(v.message)}</p>{/if}
      {#if marks}{@render marks()}{/if}
    </div>
    {#if actions}<div class="acts">{@render actions()}</div>{/if}
  </header>
  <VersionChanges {files} {error} remember={at} bind:picked>
    {#snippet viewer(cf)}
      {@const f = files?.find((x) => x.path === cf.path) ?? current!}
      {@const View = viewerFor(f).component}
      {@const s = sides(f)}
      <View root={at} file={f} a={s.a} b={s.b} compare={true} stamp={0} />
    {/snippet}
  </VersionChanges>
</div>

<style>
  .version { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  header { display: flex; align-items: flex-start; flex-wrap: wrap; gap: var(--sp-10) var(--sp-12); padding: var(--sp-14) var(--sp-16);
    border-bottom: var(--border-width) solid var(--line); }
  /* the title keeps its room: short of it, the buttons go on a line below */
  .title { flex: 1 1 280px; min-width: 0; }
  h2 { margin: 0; font-size: var(--fs-lg); font-weight: var(--fw-semibold); user-select: text; }
  .meta { font-size: var(--fs-sm); color: var(--faint); margin-top: var(--sp-2); }
  .steps-btn { display: inline-flex; align-items: center; gap: var(--sp-6); margin-left: var(--sp-8); padding: 0 var(--sp-8); line-height: 20px;
    border-radius: var(--radius-pill); border: var(--border-width) solid var(--line); background: var(--panel-2); color: var(--muted);
    font-size: var(--fs-sm); vertical-align: middle; cursor: pointer; }
  .steps-btn:hover { color: var(--text); border-color: var(--faint); }
  .steps-btn .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--faint); }
  .meta :global(.agent) { margin-right: var(--sp-4); }
  .details { margin: var(--sp-8) 0 0; font-size: var(--fs-md); color: var(--muted); white-space: pre-wrap; overflow-wrap: anywhere;
    max-height: 9em; overflow: auto; user-select: text; }
  .acts { display: flex; gap: var(--sp-6); flex-wrap: wrap; }
  .acts :global(button) { padding: var(--sp-4) var(--sp-10); font-size: var(--fs-md); }
</style>
