<script lang="ts">
  import { t } from "./i18n.svelte";
  import type { TeamProject } from "./api";
  import { cssColor, initial, projectColor } from "./palette";
  import { projectIcons } from "./projectIcons";
  import { emojiOf } from "./emoji";

  // A project's icon: the one the team chose for it (projectIcons.ts), else
  // its initial, on its colour (the team's pick, else one picked from its
  // name: the same every time); where it is shows as a small mark (on the
  // team only, folder missing). One place to change when projects get
  // pictures of their own.
  let { p, size = 22 }: {
    p: Pick<TeamProject, "name" | "status"> & { id?: string; icon?: string; color?: string }; size?: number;
  } = $props();

  // (Object.hasOwn: a name from the team's records is never a built-in key.)
  let icon = $derived(p.icon && Object.hasOwn(projectIcons, p.icon) ? projectIcons[p.icon] : "");
  // An emoji has its own colours: shown as it is, on no colour.
  let emoji = $derived(emojiOf(p.icon));
</script>

<span class="pi {p.status}" class:emoji style:--c={cssColor(projectColor(p.id || p.name, p.color))} style:--s="{size}px" aria-hidden="true">
  {#if emoji}
    <span class="e">{emoji}</span>
  {:else if icon}
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">{@html icon}</svg>
  {:else}
    {initial(p.name)}
  {/if}
  {#if p.status === "remote"}<span class="mark" title={t("On the team, not on this computer yet")}>☁</span>
  {:else if p.status === "missing"}<span class="mark warn">⚠</span>{/if}
</span>

<style>
  .pi { position: relative; flex: none; width: var(--s); height: var(--s); border-radius: calc(var(--s) * .28);
    display: inline-flex; align-items: center; justify-content: center; font-size: calc(var(--s) * .5);
    font-weight: var(--fw-bold); line-height: 1; color: var(--c); background: color-mix(in srgb, var(--c) 20%, var(--panel)); }
  .pi svg { width: 62%; height: 62%; }
  .pi.emoji { background: transparent; }
  .e { font-size: calc(var(--s) * .78); line-height: 1; font-weight: normal;
    font-family: "Segoe UI Emoji", "Apple Color Emoji", "Noto Color Emoji", sans-serif; }
  .pi.remote { opacity: .6; }
  .mark { position: absolute; right: -4px; bottom: -4px; font-size: 9px; line-height: 1; padding: 1px 2px;
    border-radius: var(--radius-pill); background: var(--bg-sunken); color: var(--muted); }
  .mark.warn { color: var(--warn); }
</style>
