<script lang="ts">
  import { t } from "./i18n.svelte";
  import { formatBytes } from "./api";
  import { queue, type Preupload } from "./preupload.svelte";

  // A small moving cloud while a big file of the project goes up in the
  // background: what, how big, how fast and how far on hover; a click opens
  // the upload queue.
  let { p }: { p: Preupload } = $props();
  let file = $derived(p.path.slice(p.path.lastIndexOf("/") + 1));
  let pct = $derived(p.total ? Math.round((100 * p.bytes) / p.total) : 0);
  let tip = $derived([
    t("Uploading in the background"),
    `${file} · ${formatBytes(p.bytes)} / ${formatBytes(p.total)} · ${pct}%${p.speed ? ` · ${formatBytes(p.speed)}/s` : ""}`,
    p.waiting.length ? t("{count} more waiting", { count: p.waiting.length }) : "",
    t("Click to see the upload queue"),
  ].filter(Boolean).join("\n"));

  function open(e: Event) {
    e.stopPropagation();
    e.preventDefault();
    queue.open = true;
  }
</script>

<button type="button" class="pre" aria-label={tip} title={tip} onclick={open}>
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M7 18a4.5 4.5 0 0 1-.6-8.96A6 6 0 0 1 18 8.5a4 4 0 0 1-.5 9.5"/>
    <g class="arrow"><path d="M12 20v-7"/><path d="m9 15 3-3 3 3"/></g>
  </svg>
</button>

<style>
  .pre { display: inline-flex; margin-left: var(--sp-4); padding: 1px; border: none; border-radius: var(--radius-sm); background: transparent;
    color: var(--ok); vertical-align: middle; cursor: pointer; }
  .pre:hover:not(:disabled) { background: var(--hover); }
  .pre:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
  svg { width: 14px; height: 14px; overflow: visible; }
  .arrow { animation: rise 1.4s ease-in-out infinite; }
  @keyframes rise {
    0% { transform: translateY(3px); opacity: 0; }
    40% { opacity: 1; }
    100% { transform: translateY(-3px); opacity: 0; }
  }
</style>
