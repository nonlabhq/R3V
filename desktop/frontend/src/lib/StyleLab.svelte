<script lang="ts">
  // The Style lab (Nightly only): try looks and change the main tokens live.
  // Ctrl+Alt+L shows or hides it; what is chosen stays on this computer.
  import { onMount } from "svelte";
  import { themes, fonts, tokens, css, load, save, apply, type Knobs } from "./themes";

  const saved = load();
  let themeId = $state(saved?.theme ?? "r3v");
  let knobs = $state<Knobs>(saved?.knobs ?? { ...themes[0].knobs });
  let open = $state(saved?.open ?? false);
  let used = $state(saved !== null); // nothing is put on the page until the lab is used
  let copied = $state(false);

  const theme = $derived(themes.find((t) => t.id === themeId) ?? themes[0]);
  const vars = $derived(tokens(theme, knobs));

  $effect(() => {
    if (!used) return;
    apply(vars);
    save({ theme: themeId, knobs: $state.snapshot(knobs), open });
  });

  function pick(id: string) {
    used = true;
    themeId = id;
    knobs = { ...(themes.find((t) => t.id === id) ?? themes[0]).knobs };
  }

  async function copy() {
    await navigator.clipboard.writeText(css(themeId, vars));
    copied = true;
    setTimeout(() => (copied = false), 1500);
  }

  onMount(() => {
    const key = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.altKey && e.key.toLowerCase() === "l") {
        e.preventDefault();
        open = !open;
        used = true;
      }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  });

  type Range = { key: keyof Knobs; label: string; min: number; max: number; step: number; unit?: string };
  const ranges: Range[] = [
    { key: "density", label: "Spacing", min: 0.7, max: 1.4, step: 0.05, unit: "×" },
    { key: "radius", label: "Corners", min: 0, max: 18, step: 1, unit: "px" },
    { key: "border", label: "Border width", min: 0, max: 3, step: 0.5, unit: "px" },
    { key: "shadow", label: "Shadow", min: 0, max: 2, step: 0.1, unit: "×" },
    { key: "surfaceAlpha", label: "Menu opacity", min: 0.3, max: 1, step: 0.05 },
    { key: "blur", label: "Menu blur", min: 0, max: 40, step: 1, unit: "px" },
    { key: "scrimBlur", label: "Dialog backdrop blur", min: 0, max: 16, step: 1, unit: "px" },
  ];
</script>

{#if open}
  <div class="lab" role="dialog" aria-label="Style lab">
    <div class="head">
      <strong>Style lab</strong>
      <span class="hint">Ctrl+Alt+L</span>
      <button class="ghost x" onclick={() => (open = false)} aria-label="Close">✕</button>
    </div>
    <div class="themes">
      {#each themes as t (t.id)}
        <button class:on={t.id === themeId} title={t.note} onclick={() => pick(t.id)}>{t.name}</button>
      {/each}
    </div>
    <p class="note">{theme.note}</p>
    {#each ranges as r (r.key)}
      <label class="knob">
        <span>{r.label}</span>
        <input type="range" min={r.min} max={r.max} step={r.step} bind:value={knobs[r.key] as number}
          oninput={() => (used = true)} />
        <span class="val">{knobs[r.key]}{r.unit ?? ""}</span>
      </label>
    {/each}
    <label class="knob">
      <span>Accent</span>
      <input type="color" bind:value={knobs.accent} oninput={() => (used = true)} />
      <span class="val">{knobs.accent}</span>
    </label>
    <label class="knob">
      <span>Border colour</span>
      <input type="color" bind:value={knobs.line} oninput={() => (used = true)} />
      <span class="val">{knobs.line}</span>
    </label>
    <label class="knob">
      <span>Font</span>
      <select bind:value={knobs.font} onchange={() => (used = true)}>
        {#each fonts as f (f.name)}<option value={f.value}>{f.name}</option>{/each}
      </select>
      <span></span>
    </label>
    <div class="foot">
      <button onclick={() => pick(themeId)}>Reset</button>
      <button onclick={copy}>{copied ? "Copied" : "Copy as CSS"}</button>
    </div>
  </div>
{/if}

<style>
  /* The lab keeps a fixed size so it stays put while spacing changes. */
  .lab { position: fixed; right: 16px; bottom: 16px; z-index: var(--z-blocking); width: 300px; padding: 12px;
    background: var(--panel-2); border: 1px solid var(--line-strong); border-radius: 8px; box-shadow: var(--shadow-pop);
    font-size: 12px; user-select: none; }
  .head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
  .hint { color: var(--faint); flex: 1; }
  .x { padding: 0 6px; }
  .themes { display: grid; grid-template-columns: repeat(5, 1fr); gap: 4px; }
  .themes button { padding: 4px 0; font-size: 12px; border-radius: 6px; }
  .themes button.on { border-color: var(--accent); color: var(--accent); }
  .note { margin: 6px 0 10px; color: var(--muted); }
  .knob { display: grid; grid-template-columns: 96px 1fr 56px; align-items: center; gap: 8px; margin: 4px 0;
    font-size: 12px; color: var(--muted); }
  .knob input[type="range"] { padding: 0; accent-color: var(--accent); }
  .knob input[type="color"] { padding: 0; height: 22px; border-radius: 4px; }
  .knob select { font: inherit; color: var(--text); background: var(--bg); border: 1px solid var(--line); border-radius: 4px; padding: 2px 4px; }
  .val { text-align: right; color: var(--text); font-variant-numeric: tabular-nums; }
  .foot { display: flex; gap: 6px; justify-content: flex-end; margin-top: 10px; }
  .foot button { padding: 4px 10px; font-size: 12px; }
</style>
