<script lang="ts">
  import TextViewer from "$shared/viewers/TextViewer.svelte";
  import { appSource } from "./appSource";
  import type { ViewerProps } from "./types";

  // Text and code (the shared viewer, on this project's files).
  let { root, file, a, b, compare, stamp }: ViewerProps = $props();
  const source = $derived(appSource(root, stamp));
</script>

<TextViewer {source} {file} {a} {b} {compare} {stamp} />
