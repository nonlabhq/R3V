<script lang="ts">
  import { t } from "./i18n.svelte";
  import type { TeamProject } from "./api";

  // A project's icon: for now its initial on a colour of its own (the same
  // every time); where it is shows as a small mark (on the team only,
  // folder missing). One place to change when projects get icons of their own.
  let { p, size = 22 }: { p: TeamProject; size?: number } = $props();

  const initial = (name: string) => ([...name.trim()][0] ?? "?").toUpperCase();
  // A lane colour picked from the name.
  // (not lane 0: that is the main branch's colour, the brand's)
  let hue = $derived(1 + ([...p.name].reduce((h, c) => (h * 31 + c.codePointAt(0)!) >>> 0, 7) % 4));
</script>

<span class="pi {p.status}" style:--c="var(--lane-{hue})" style:--s="{size}px" aria-hidden="true">
  {initial(p.name)}
  {#if p.status === "remote"}<span class="mark" title={t("On the team, not on this computer yet")}>☁</span>
  {:else if p.status === "missing"}<span class="mark warn">⚠</span>{/if}
</span>

<style>
  .pi { position: relative; flex: none; width: var(--s); height: var(--s); border-radius: calc(var(--s) * .28);
    display: inline-flex; align-items: center; justify-content: center; font-size: calc(var(--s) * .5);
    font-weight: var(--fw-bold); line-height: 1; color: var(--c); background: color-mix(in srgb, var(--c) 20%, var(--panel)); }
  .pi.remote { opacity: .6; }
  .mark { position: absolute; right: -4px; bottom: -4px; font-size: 9px; line-height: 1; padding: 1px 2px;
    border-radius: var(--radius-pill); background: var(--bg-sunken); color: var(--muted); }
  .mark.warn { color: var(--warn); }
</style>
