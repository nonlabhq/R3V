<script lang="ts">
  import { t } from "./i18n.svelte";
  import { toolName } from "./api";

  // An AI agent's mark: a small spark with the tool's name ("Claude Code"),
  // quiet beside a version or a branch. compact: the spark alone (its name
  // in the tooltip). title: the tooltip instead of who made it.
  let { tool, model = "", compact = false, title = "" }: { tool: string; model?: string; compact?: boolean; title?: string } = $props();
  let name = $derived(toolName(tool) || t("An agent"));
  let tip = $derived(title || (model ? t("Made by {tool} ({model})", { tool: name, model }) : t("Made by {tool}", { tool: name })));
</script>

<span class="agent" class:compact title={tip} aria-label={tip}>
  <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 1.5 9.4 6.6 14.5 8 9.4 9.4 8 14.5 6.6 9.4 1.5 8 6.6 6.6z" fill="currentColor" /></svg>
  {#if !compact}<span class="name">{name}</span>{/if}
</span>

<style>
  .agent { flex: none; display: inline-flex; align-items: center; gap: var(--sp-4); padding: 0 var(--sp-6) 0 var(--sp-4);
    height: 18px; border-radius: var(--radius-pill); background: var(--info-chip); color: var(--info-text);
    font-size: var(--fs-2xs); font-weight: var(--fw-semibold); white-space: nowrap; vertical-align: middle; line-height: 1; }
  .agent.compact { padding: 0; width: 16px; height: 16px; justify-content: center; }
  svg { width: 10px; height: 10px; flex: none; }
  .name { overflow: hidden; text-overflow: ellipsis; }
</style>
