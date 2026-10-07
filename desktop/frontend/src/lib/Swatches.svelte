<script lang="ts">
  import { t } from "./i18n.svelte";
  import { palette, cssColor } from "./palette";

  // The palette to pick a colour from (tokens.css --palette-*).
  let { value, label, onpick }: { value: string; label: string; onpick: (name: string) => void } = $props();

  function nameOf(c: string): string {
    switch (c) {
      case "orange": return t("Orange");
      case "amber": return t("Amber");
      case "lime": return t("Lime");
      case "green": return t("Green");
      case "teal": return t("Teal");
      case "cyan": return t("Cyan");
      case "blue": return t("Blue");
      case "indigo": return t("Indigo");
      case "violet": return t("Violet");
      case "pink": return t("Pink");
      case "red": return t("Red");
      case "sand": return t("Sand");
      case "slate": return t("Slate");
    }
    return c;
  }
</script>

<div class="swatches" role="radiogroup" aria-label={label}>
  {#each palette as c (c)}
    <button class="sw" class:on={value === c} style:--c={cssColor(c)} role="radio" aria-checked={value === c}
      aria-label={nameOf(c)} title={nameOf(c)} onclick={() => onpick(c)}></button>
  {/each}
</div>

<style>
  .swatches { display: flex; flex-wrap: wrap; gap: var(--sp-6); }
  .sw { width: 22px; height: 22px; padding: 0; border-radius: 50%; border: 2px solid transparent; background: var(--c);
    box-shadow: inset 0 0 0 2px var(--panel); }
  .sw:hover:not(:disabled) { background: var(--c); border-color: var(--line-strong); }
  .sw.on { border-color: var(--c); }
  .sw:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
</style>
