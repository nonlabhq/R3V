<script lang="ts">
  import { t } from "./i18n";
  import type { ProjectLook } from "./types";
  import { cssColor, initial, projectColor } from "./palette";
  import { projectIcons } from "./projectIcons";
  import { emojiOf } from "./emoji";

  // A project's icon (the team's too: TeamMark): its own picture, else the
  // icon the team chose for it (projectIcons.ts), else its initial, on its
  // colour (the team's pick, else one picked from its name: the same every
  // time); where it is shows as a small mark (on the team only, folder
  // missing).
  // plain: no colour of its own picked, a light mark (a team's).
  let { p, size = 22, plain = false }: { p: ProjectLook; size?: number; plain?: boolean } = $props();
  let broken = $state(""); // a picture that can't be shown: the icon instead

  // (Object.hasOwn: a name from the team's records is never a built-in key.)
  let icon = $derived(p.icon && Object.hasOwn(projectIcons, p.icon) ? projectIcons[p.icon] : "");
  // An emoji has its own colours: shown as it is, on no colour.
  let emoji = $derived(emojiOf(p.icon));
  let pic = $derived(!!p.picture && p.picture !== broken);
</script>

<span class="pi {p.status}" class:plain={plain && !p.color} class:emoji={!!emoji && !pic} class:pic style:--c={cssColor(projectColor(p.id || p.name, p.color))} style:--s="{size}px" aria-hidden="true">
  {#if pic}
    <img src={p.picture} alt="" draggable="false" onerror={() => (broken = p.picture ?? "")} />
  {:else if emoji}
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
    display: inline-flex; align-items: center; justify-content: center; font-size: calc(var(--s) * .44);
    font-family: var(--font-brand); font-weight: var(--fw-bold); line-height: 1; color: var(--palette-ink); background: var(--c); }
  .pi.plain { background: var(--text); }
  .pi svg { width: 62%; height: 62%; }
  .pi.emoji, .pi.pic { background: transparent; }
  .pi img { width: 100%; height: 100%; border-radius: inherit; object-fit: cover; }
  .e { font-size: calc(var(--s) * .78); line-height: 1; font-weight: normal;
    font-family: "Segoe UI Emoji", "Apple Color Emoji", "Noto Color Emoji", sans-serif; }
  .pi.remote { opacity: .45; }
  .mark { position: absolute; right: -4px; bottom: -4px; font-size: 9px; line-height: 1; padding: 1px 2px;
    border-radius: var(--radius-pill); background: var(--bg-sunken); color: var(--muted); }
  .mark.warn { color: var(--warn); }
</style>
