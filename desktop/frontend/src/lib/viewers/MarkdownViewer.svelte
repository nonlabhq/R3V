<script lang="ts">
  import MarkdownViewer from "$shared/viewers/MarkdownViewer.svelte";
  import { api } from "../api";
  import { appSource } from "./appSource";
  import type { ViewerProps } from "./types";

  // Markdown notes (the shared viewer); their links open in the browser.
  let { root, file, a, b, compare, stamp }: ViewerProps = $props();
  const source = $derived(appSource(root, stamp));
</script>

<MarkdownViewer {source} {file} {a} {b} {compare} {stamp} onlink={(href) => api.OpenURL(href).catch(() => {})} />
