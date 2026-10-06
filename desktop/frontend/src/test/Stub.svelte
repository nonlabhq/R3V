<script lang="ts">
  // Stands in for a heavy component in tests (e.g. ProjectView in App.test.ts).
  let { root = "" }: { root?: string } = $props();
</script>

<p data-testid="stub">{root}</p>
