<script lang="ts">
  import { t } from "../i18n";
  import VideoCompare from "./VideoCompare.svelte";
  import { opened, type Opened } from "./opened.svelte";
  import type { Side, ViewerProps } from "./source";

  // A video to watch; comparing, the other beside it, played together.
  let { source, a, b, compare, onopen }: ViewerProps = $props();
  const oa = opened(() => source, () => a);
  const ob = opened(() => source, () => b);
  const take = (s: Side | null, o: Opened) => (s && o.file ? { label: s.label, src: o.file.url } : null);
  let error = $derived(oa.error || ob.error);
  let ready = $derived((!a || !!oa.file) && (!b || !!ob.file));
</script>

{#if error === "tooBig"}
  <p class="muted small">{t("Too big to show here.")}</p>
{:else if error}
  <p class="muted small">{t("Couldn't read it:")} {error}</p>
{:else if !ready}
  <p class="muted small">{t("Reading…")}</p>
{:else}
  <VideoCompare a={take(a, oa)} b={take(b, ob)} {compare} {onopen} />
{/if}

<style>
  .small { font-size: var(--fs-sm); }
</style>
