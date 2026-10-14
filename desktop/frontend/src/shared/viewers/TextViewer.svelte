<script lang="ts">
  import TextView from "./TextView.svelte";
  import type { ViewerProps } from "./source";

  // Text and code: the whole file; comparing, its changes line by line.
  let { source, a, b, compare, stamp = 0 }: ViewerProps = $props();
</script>

<TextView {source} {stamp} {compare} file={(a ?? b)!.path} fromFile={b?.path ?? ""}
  from={b ? b.version : "none"} to={a ? a.version : "none"} />
