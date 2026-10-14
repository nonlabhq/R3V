<script lang="ts">
  import { colorOf, cssColor, initial } from "./palette";

  // A person: their picture, else their initial on their colour (their
  // pick, else one picked from seed, their member id: the same for
  // everyone).
  let { name, seed = "", color = "", picture = "", size = 24 }: {
    name: string; seed?: string; color?: string; picture?: string; size?: number;
  } = $props();

  // A picture that can't be shown (bad data from the team): the initial.
  let failed = $state("");
</script>

<span class="av" style:--c={cssColor(colorOf(color, seed || name))} style:--s="{size}px" aria-hidden="true">
  {#if picture && picture !== failed}<img src={picture} alt="" draggable="false" onerror={() => (failed = picture)} />{:else}{initial(name)}{/if}
</span>

<style>
  .av { flex: none; width: var(--s); height: var(--s); border-radius: 50%; overflow: hidden; display: inline-flex;
    align-items: center; justify-content: center; font-size: calc(var(--s) * .46); font-weight: var(--fw-semibold);
    line-height: 1; color: var(--c); background: color-mix(in srgb, var(--c) 22%, var(--panel)); }
  img { width: 100%; height: 100%; object-fit: cover; display: block; }
</style>
