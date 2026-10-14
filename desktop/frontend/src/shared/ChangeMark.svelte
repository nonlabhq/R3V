<script lang="ts" module>
  import { t } from "./i18n";

  // What happened to a file, as one letter the same everywhere (Your
  // changes, a version's files, the Files tab): A new, C changed, D
  // deleted, M moved; U no longer tracked.
  const letters: Record<string, string> = { added: "A", modified: "C", deleted: "D", renamed: "M", untracked: "U" };

  /** The letter for a status ("" for none: unchanged, ignored). */
  export const changeLetter = (status: string) => letters[status] ?? "";

  /** The status in words, for its tooltip. */
  export const changeName = (status: string) => ({ added: t("New"), modified: t("Changed"), deleted: t("Deleted"), renamed: t("Moved"),
    untracked: t("No longer tracked: the rules leave it out now") } as Record<string, string>)[status] ?? "";
</script>

<script lang="ts">
  let { status, title }: { status: string; title?: string } = $props();
</script>

{#if changeLetter(status)}
  <span class="mark {status}" title={title ?? changeName(status)} aria-label={title ?? changeName(status)}>{changeLetter(status)}</span>
{/if}

<style>
  .mark { flex: none; display: inline-flex; align-items: center; justify-content: center; width: 16px; height: 16px;
    border-radius: var(--radius-sm); font-size: var(--fs-2xs); font-weight: var(--fw-bold); line-height: 1;
    background: var(--hover); color: var(--muted); }
  .added { background: var(--change-added-soft); color: var(--change-added); }
  .modified { background: var(--change-changed-soft); color: var(--change-changed); }
  .deleted { background: var(--change-deleted-soft); color: var(--change-deleted); }
  .renamed { background: var(--change-moved-soft); color: var(--change-moved); }
</style>
