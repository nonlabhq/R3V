<script lang="ts">
  import ModelViewer from "$shared/viewers/ModelViewer.svelte";
  import { appSource } from "./appSource";
  import type { ViewerProps } from "./types";

  // 3D models (the shared viewer). Now's model is read again when its size
  // changes, not with every reload of the list: the view (and where you
  // turned it) stays.
  let { root, file, a, b, compare }: ViewerProps = $props();
  const source = $derived(appSource(root, `s${file.size}`));
</script>

<ModelViewer {source} {file} {a} {b} {compare} />
