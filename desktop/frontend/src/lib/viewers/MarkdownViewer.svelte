<script lang="ts">
  import { t } from "../i18n.svelte";
  import { api, errorText } from "../api";
  import TextViewer from "./TextViewer.svelte";
  import { renderMarkdown } from "./markdown";
  import { mdLook, setMdText } from "./settings.svelte";
  import type { ViewerProps } from "./types";

  // Markdown notes (README, mix notes…): as they read, or as text; their
  // changes line by line, as text.
  let props: ViewerProps = $props();
  let side = $derived(props.a ?? props.b);
  let html = $state("");
  let error = $state("");
  $effect(() => {
    const s = side, stamp = props.stamp;
    if (props.compare || mdLook.text || !s) return;
    void stamp;
    error = "";
    api.TextFile(props.root, s.path, s.version).then((c) => {
      if (side !== s) return;
      if (!c?.text) { html = ""; error = c?.tooBig ? t("Too big to show here.") : t("Not a text file."); return; }
      html = renderMarkdown(c.lines.join("\n"));
    })
      .catch((e) => { if (side === s) error = errorText(e); });
  });
  // Links open in the browser (never in the app's page).
  function click(e: MouseEvent) {
    const a = (e.target as HTMLElement).closest<HTMLElement>("a[data-href]");
    if (!a) return;
    e.preventDefault();
    api.OpenURL(a.dataset.href!).catch(() => {});
  }
</script>

{#if props.compare}
  <TextViewer {...props} />
{:else}
  <div class="md-view">
    <div class="look" role="group" aria-label={t("Display")}>
      <button class:on={!mdLook.text} aria-pressed={!mdLook.text} onclick={() => setMdText(false)}>{t("Preview")}</button>
      <button class:on={mdLook.text} aria-pressed={mdLook.text} onclick={() => setMdText(true)}>{t("Text")}</button>
    </div>
    {#if mdLook.text}
      <TextViewer {...props} />
    {:else if error}
      <p class="muted">{error}</p>
    {:else}
      <!-- (escaped and made from a few tags only: see markdown.ts) -->
      <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
      <article class="md" onclick={click}>{@html html}</article>
    {/if}
  </div>
{/if}

<style>
  .md-view { display: flex; flex-direction: column; gap: var(--sp-8); min-width: 0; }
  .look { align-self: flex-start; display: flex; padding: 2px; border-radius: var(--radius-pill); background: var(--bg-sunken); }
  .look button { border: none; border-radius: var(--radius-pill); background: transparent; padding: var(--sp-2) var(--sp-10); font-size: var(--fs-sm); color: var(--muted); }
  .look button.on { background: var(--text); color: var(--bg); }
  .md { font-size: var(--fs-md); line-height: 1.6; color: var(--text); overflow-wrap: anywhere; user-select: text; }
  .md :global(h1), .md :global(h2), .md :global(h3), .md :global(h4), .md :global(h5), .md :global(h6) { margin: var(--sp-14) 0 var(--sp-6); line-height: 1.3; }
  .md :global(h1) { font-size: var(--fs-xl); }
  .md :global(h2) { font-size: var(--fs-lg); }
  .md :global(h3), .md :global(h4), .md :global(h5), .md :global(h6) { font-size: var(--fs-md); }
  .md :global(:first-child) { margin-top: 0; }
  .md :global(p), .md :global(ul), .md :global(ol), .md :global(blockquote), .md :global(pre), .md :global(table) { margin: 0 0 var(--sp-10); }
  .md :global(ul), .md :global(ol) { padding-left: var(--sp-20); }
  .md :global(li.task) { list-style: none; margin-left: calc(var(--sp-16) * -1); }
  .md :global(li.task input) { width: auto; margin: 0 var(--sp-4) 0 0; vertical-align: -1px; }
  .md :global(blockquote) { padding: var(--sp-2) var(--sp-12); border-left: 3px solid var(--line-strong); color: var(--muted); }
  .md :global(code) { font-family: var(--font-mono); font-size: .9em; padding: 0 var(--sp-4); border-radius: var(--radius-sm); background: var(--hover); }
  .md :global(pre) { padding: var(--sp-10) var(--sp-12); border-radius: var(--radius); background: var(--bg-sunken); overflow: auto; }
  .md :global(pre code) { padding: 0; background: none; }
  .md :global(hr) { border: none; border-top: var(--border-width) solid var(--line); margin: var(--sp-14) 0; }
  .md :global(table) { border-collapse: collapse; font-size: var(--fs-sm); }
  .md :global(th), .md :global(td) { padding: var(--sp-4) var(--sp-10); border: var(--border-width) solid var(--line); text-align: left; }
  .md :global(th) { background: var(--hover); }
  .md :global(a) { color: var(--accent-text); }
</style>
