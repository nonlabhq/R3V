<script lang="ts">
  import VideoViewer from "$shared/viewers/VideoViewer.svelte";
  import { api, errorText } from "../api";
  import { toast } from "../notify.svelte";
  import { appSource } from "./appSource";
  import type { ViewerProps } from "./types";

  // Videos (the shared viewer); the one in the folder opens in its program.
  let { root, file, a, b, compare, stamp }: ViewerProps = $props();
  const source = $derived(appSource(root, stamp));
  const shown = $derived(a ?? b);
</script>

<VideoViewer {source} {file} {a} {b} {compare}
  onopen={shown && !shown.version ? () => api.OpenInLive(root, shown.path).catch((e) => toast(errorText(e), "error")) : undefined} />
