<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, ago, type StepsInfo } from "./api";
  import StepsList from "./StepsList.svelte";

  // The steps of the work you're on (docs/design/steps.md): a pill beside
  // the branch menu, there whenever this copy keeps steps, lit while an
  // agent is making them; it opens the session's prompts and steps, to go
  // back to one or compare. Read again when the project changes (version),
  // and now and then: agents work without telling the app.
  let { root, version, busy = false, onback }: {
    root: string;
    version: string;
    busy?: boolean;
    onback: (session: string, n: number) => void; // go back to step n (the view runs it: Live's check, the result)
  } = $props();

  let info = $state<StepsInfo | null>(null);
  let session = $state(""); // "" the latest
  let open = $state(false);
  let now = $state(Date.now());

  async function load() {
    const r = root;
    try {
      const got = await api.Steps(r, session);
      if (r === root) info = got;
    } catch {
      // (no steps to show: the pill stays as it was)
    }
    now = Date.now();
  }
  $effect(() => {
    root; version; session;
    load();
  });
  $effect(() => {
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
  });

  const s = $derived(info?.session ?? null);
  const latestPrompt = $derived.by(() => {
    for (const turn of [...(s?.turns ?? [])].reverse()) if (turn.prompt || turn.note) return turn.prompt || turn.note;
    return "";
  });
  // Steps made in the last few minutes: an agent is at work.
  const live = $derived(!!s && now - Date.parse(s.last) < 10 * 60_000);

  // Esc, or a click anywhere else, closes it.
  let box = $state<HTMLElement>();
  function onkey(e: KeyboardEvent) {
    if (open && e.key === "Escape") open = false;
  }
  function ondown(e: PointerEvent) {
    if (open && box && !box.contains(e.target as Node) && !(e.target as Element).closest?.(".modal")) open = false;
  }
</script>

<svelte:window onkeydown={onkey} onpointerdown={ondown} />

{#if info?.available && info.on && s}
  <div class="steps" bind:this={box}>
    <button class="pill" class:live onclick={() => (open = !open)} aria-expanded={open}
      title={live ? t("R3V is keeping a step after each of the agent's moves") : t("Steps of the work here")}>
      <span class="dot" aria-hidden="true"></span>
      {live ? t("Recording steps") : t("Steps")} · {s.steps}
    </button>
    {#if open}
      <div class="panel" role="dialog" aria-label={t("Steps")}>
        <div class="head">
          <strong>{t("Steps")}</strong>
          <span class="faint">{tn(s.steps, "{n} step", "{n} steps")} · {ago(s.last)}{latestPrompt ? ` · “${latestPrompt}”` : ""}</span>
        </div>
        {#if (info.sessions?.length ?? 0) > 1}
          <label class="session">
            <span class="faint">{t("Session")}</span>
            <select bind:value={session}>
              <option value="">{t("The latest")}</option>
              {#each info.sessions as o (o.id)}
                <option value={o.id}>{o.prompt ? o.prompt.slice(0, 60) : o.id} · {ago(o.last)}</option>
              {/each}
            </select>
          </label>
        {/if}
        <div class="list"><StepsList {root} session={s} {busy} onback={(id, n) => { open = false; onback(id, n); }} /></div>
      </div>
    {/if}
  </div>
{/if}

<style>
  .steps { position: relative; }
  .pill { display: inline-flex; align-items: center; gap: var(--sp-6); padding: var(--sp-4) var(--sp-10); border-radius: var(--radius-pill);
    border: var(--border-width) solid var(--line); background: var(--panel-2); color: var(--muted); font-size: var(--fs-sm); cursor: pointer; }
  .pill.live { color: var(--text); border-color: var(--accent); }
  .dot { width: 7px; height: 7px; border-radius: 50%; background: var(--faint); }
  .pill.live .dot { background: var(--accent); animation: pulse 1.6s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: .35; } }
  @media (prefers-reduced-motion: reduce) { .pill.live .dot { animation: none; } }
  .panel { position: absolute; top: calc(100% + var(--sp-6)); left: 0; z-index: var(--z-menu); width: min(560px, 70vw);
    max-height: 60vh; display: flex; flex-direction: column; padding: var(--sp-10) var(--sp-12); border-radius: var(--radius-lg);
    background: var(--surface-menu); backdrop-filter: var(--surface-filter); box-shadow: var(--shadow-pop);
    border: var(--border-width) solid var(--line); }
  .head { display: flex; align-items: baseline; gap: var(--sp-8); overflow: hidden; white-space: nowrap; }
  .head .faint { overflow: hidden; text-overflow: ellipsis; font-size: var(--fs-sm); }
  .session { display: flex; align-items: center; gap: var(--sp-8); margin: var(--sp-6) 0 0; }
  .list { overflow-y: auto; min-height: 0; }
</style>
