<script lang="ts">
  import { progressDetail, progressFraction, progressText, type Progress } from "./api";

  // A long step: what is happening, files, bytes, speed and time left.
  // Without p (not started yet) it shows `waiting` and a moving bar.
  let { p, team, waiting = "Working…" }: { p: Progress | null; team?: string; waiting?: string } = $props();

  let detail = $derived(p ? progressDetail(p) : "");
  let known = $derived(!!p && (!!p.totalBytes || !!p.total));

  // Still working: a step that reports no progress for a while (the end
  // of an upload, a request to the team) keeps a gradient moving over the
  // bar, so it never looks stuck.
  const STILL = 1500;
  let changed = $state(Date.now());
  let still = $state(false);
  let sig = $derived(p ? `${p.stage}:${p.done}:${p.bytes}` : "");
  $effect(() => {
    sig;
    changed = Date.now();
    still = false;
  });
  $effect(() => {
    const timer = setInterval(() => (still = Date.now() - changed >= STILL), 500);
    return () => clearInterval(timer);
  });
</script>

<div class="progress">
  <div class="text">
    <span>{p ? progressText(p, team) : waiting}</span>
    {#if detail}<span class="faint detail">{detail}</span>{/if}
  </div>
  <div class="bar" class:indeterminate={!known} class:still={known && still} data-testid="progress-bar">
    <div style="width: {known ? Math.round(progressFraction(p!) * 100) : 30}%"></div>
  </div>
</div>

<style>
  .progress { flex: 1; display: flex; flex-direction: column; gap: var(--sp-6); font-size: var(--fs-md); min-width: 0; }
  .text { display: flex; flex-wrap: wrap; gap: var(--sp-4) var(--sp-12); }
  .detail { font-variant-numeric: tabular-nums; }
  .bar { height: 4px; border-radius: 2px; background: var(--line); overflow: hidden; }
  .bar > div { height: 100%; background: var(--accent); transition: width .2s; }
  .bar.indeterminate > div { animation: slide 1.2s ease-in-out infinite; }
  @keyframes slide { from { transform: translateX(-100%); } to { transform: translateX(340%); } }
  .bar { position: relative; }
  .bar.still::after { content: ""; position: absolute; inset: 0;
    background: linear-gradient(90deg, transparent, color-mix(in srgb, var(--text) 35%, transparent), transparent);
    background-size: 40% 100%; background-repeat: no-repeat; animation: sweep 1.4s linear infinite; }
  @keyframes sweep { from { background-position: -40% 0; } to { background-position: 140% 0; } }
  @media (prefers-reduced-motion: reduce) { .bar.still::after, .bar.indeterminate > div { animation-duration: 3s; } }
</style>
