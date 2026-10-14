<script lang="ts">
  import ImageViewer from "$shared/viewers/ImageViewer.svelte";
  import { appSource } from "./appSource";
  import type { ViewerProps } from "./types";

  // Pictures and design files (the shared viewer, design files as pictures).
  let { root, file, a, b, compare, stamp }: ViewerProps = $props();
  const source = $derived(appSource(root, stamp));
</script>

<ImageViewer {source} {file} {a} {b} {compare} />
