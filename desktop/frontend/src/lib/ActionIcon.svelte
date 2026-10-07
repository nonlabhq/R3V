<script lang="ts">
  // The icons of what can be done with a version (Go to, Merge, …): line
  // icons in the text's colour, before the button's words.
  let { name }: { name: "goto" | "merge" | "undo" | "branch" | "export" | "flag" } = $props();
</script>

<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  {#if name === "goto"}
    <!-- into it: an arrow to a line -->
    <path d="M2.5 8h8M7.5 5l3 3-3 3M13.5 3v10" />
  {:else if name === "merge"}
    <!-- a branch joining the line -->
    <circle cx="4.5" cy="3.5" r="1.5" /><circle cx="4.5" cy="12.5" r="1.5" /><circle cx="11.5" cy="8" r="1.5" />
    <path d="M4.5 5v6M4.5 5c0 2.5 2 3 5.5 3" />
  {:else if name === "undo"}
    <path d="M5 4 2.5 6.5 5 9" /><path d="M2.5 6.5h7a3.5 3.5 0 0 1 0 7H7" />
  {:else if name === "branch"}
    <!-- a line with a new branch out of it -->
    <circle cx="4.5" cy="12.5" r="1.5" /><circle cx="4.5" cy="3.5" r="1.5" />
    <path d="M4.5 5v6M4.5 9c0-2.5 2-3.5 5-3.5" /><path d="M12 3v5M9.5 5.5h5" />
  {:else if name === "export"}
    <path d="M8 2.5v8M5 7.5l3 3 3-3M3 12.5v1h10v-1" />
  {:else if name === "flag"}
    <!-- a milestone -->
    <path d="M3.5 14V2.5" /><path d="M3.5 3h8.5l-2 3 2 3H3.5" />
  {/if}
</svg>

<style>
  svg { width: 14px; height: 14px; flex: none; vertical-align: -2px; margin-right: var(--sp-6); }
</style>
