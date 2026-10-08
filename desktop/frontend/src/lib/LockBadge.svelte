<script lang="ts">
  import type { MemberLook } from "./api";
  import Avatar from "./Avatar.svelte";
  import { holderName, lockText, type LockItem } from "./locks.svelte";

  // A lock on a file or folder: a padlock and who holds it (their picture
  // or initial), the rest in its tooltip.
  let { lock, looks, size = 16 }: { lock: LockItem; looks?: Record<string, MemberLook | undefined>; size?: number } = $props();
</script>

<span class="lock" class:mine={lock.mine} title={lockText(lock)} aria-label={lockText(lock)}>
  <svg viewBox="0 0 16 16" aria-hidden="true" style:width="{size - 4}px" style:height="{size - 4}px">
    <rect x="3" y="7" width="10" height="7" rx="1.5" fill="currentColor" />
    <path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2" fill="none" stroke="currentColor" stroke-width="1.6" />
  </svg>
  <Avatar name={lock.name || holderName(lock)} seed={lock.memberId} color={looks?.[lock.memberId]?.color ?? ""}
    picture={looks?.[lock.memberId]?.picture ?? ""} {size} />
</span>

<style>
  .lock { flex: none; display: inline-flex; align-items: center; gap: var(--sp-2); padding: 1px var(--sp-4) 1px var(--sp-2);
    border-radius: var(--radius-pill); background: var(--warn-soft); color: var(--warn); }
  .lock.mine { background: var(--accent-soft); color: var(--accent); }
</style>
