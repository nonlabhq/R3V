<script lang="ts">
  import { t } from "./i18n.svelte";
  import { palette, cssColor } from "./palette";

  // The palette to pick a colour from (tokens.css --palette-*). Disabled
  // (a pick being saved), the swatches keep the focus but take no pick.
  let { value, label, disabled = false, onpick }: {
    value: string; label: string; disabled?: boolean; onpick: (name: string) => void;
  } = $props();

  // What each number is called (the names only show; the number is kept).
  function nameOf(c: string): string {
    switch (c) {
      case "b1": return t("Violet");
      case "b2": return t("Teal");
      case "b3": return t("Blue");
      case "b4": return t("Pink");
      case "b5": return t("Lime");
      case "b6": return t("Indigo");
      case "b7": return t("Mint");
      case "b8": return t("Magenta");
      case "b9": return t("Sky");
      case "b10": return t("Green");
      case "b11": return t("Lilac");
      case "b12": return t("Cyan");
    }
    return c;
  }
</script>

<div class="swatches" role="radiogroup" aria-label={label}>
  {#each palette as c (c)}
    <button class="sw" class:on={value === c} style:--c={cssColor(c)} role="radio" aria-checked={value === c}
      aria-label={nameOf(c)} title={nameOf(c)} aria-disabled={disabled} onclick={() => disabled || onpick(c)}></button>
  {/each}
</div>

<style>
  .swatches { display: flex; flex-wrap: wrap; gap: var(--sp-6); }
  .sw { width: 22px; height: 22px; padding: 0; border-radius: 50%; border: 2px solid transparent; background: var(--c);
    box-shadow: inset 0 0 0 2px var(--panel); }
  .sw:hover:not(:disabled) { background: var(--c); border-color: var(--line-strong); }
  .sw.on { border-color: var(--text); }
  .sw[aria-disabled="true"] { cursor: progress; }
  .sw:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
</style>
