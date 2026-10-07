<script lang="ts">
  import { t } from "./i18n.svelte";
  import type { State } from "./api";

  // The commit box under the Changes list: the description, and the button
  // that says what it commits (all the changes, or the ticked ones) and
  // where it goes (this computer, or the team too).
  let { st, message = $bindable(), busy, leftOut, oncommit, inline = false }: {
    st: State;
    message: string;
    busy: string;
    leftOut: number; // changes unticked: they stay uncommitted
    oncommit: () => void;
    inline?: boolean; // one line: the message, then the button (a panel's footer)
  } = $props();

  // The commit shortcut as the keyboard says it.
  const isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
  let unticked = $derived(leftOut ? " " + t("Unticked files stay uncommitted.") : "");
</script>

<div class="box" class:inline>
<textarea rows={inline ? 1 : 3} bind:value={message} placeholder={t("What did you change? e.g. “New bassline in the chorus”")}
  onkeydown={(e) => { if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) oncommit(); }}></textarea>
{#if st.olderVersion}
  <p class="faint small older">{st.remoteUrl
    ? t("You're on an older version. Committing combines your changes with the latest version of “{branch}” (you'll see a preview first) — or start a new branch from here.", { branch: st.branch })
    : t("You're on an older version. Make it the latest version to commit changes, or go back to the latest version.")}</p>
{/if}
<button class="primary commit-btn" disabled={!message.trim() || !!busy || (!!st.olderVersion && !st.remoteUrl)
  || (st.changes.length > 0 && leftOut === st.changes.length)} onclick={oncommit}
  title={st.remoteUrl
    ? t("Commits the project folder and shares it with the team on “{branch}”. If others committed in the meantime, you'll see what they changed and choose how to combine first.", { branch: st.branch }) + unticked
    : t("Commits on this computer. Share the project with a team to work on it together.") + unticked}
  aria-keyshortcuts="Control+Enter">
  <span>{busy === "save" || busy === "first-share" ? t("Committing…")
    : leftOut ? t(st.remoteUrl ? "Commit {done} of {total} & Share" : "Commit {done} of {total}", { done: st.changes.length - leftOut, total: st.changes.length })
    : st.remoteUrl ? t("Commit & Share") : t("Commit")}</span>
  <kbd>{isMac ? "⌘" : "Ctrl"} ↵</kbd>
</button>
</div>

<style>
  .inline { display: grid; grid-template-columns: 1fr auto; align-items: center; gap: var(--sp-10); }
  .inline textarea { grid-column: 1; grid-row: 1; resize: none; min-height: 0; border: none; border-radius: var(--radius);
    background: rgba(255, 255, 255, .07); padding: var(--sp-8) var(--sp-12); }
  .inline .commit-btn { grid-column: 2; grid-row: 1; width: auto; margin: 0; white-space: nowrap; }
  .inline .older { grid-column: 1 / -1; }
  .small { font-size: var(--fs-sm); margin: var(--sp-10) 0 0; }
  .older { margin: var(--sp-6) 0 0; }
  .commit-btn { width: 100%; margin-top: var(--sp-8); padding: var(--sp-8) var(--sp-14); display: flex; align-items: center; justify-content: center; gap: var(--sp-10); }
  .commit-btn kbd { font: inherit; font-size: var(--fs-xs); opacity: .75; padding: 1px var(--sp-4); border-radius: var(--radius-sm); border: var(--border-width) solid currentColor; }
  /* nothing to commit yet (no message, no changes): outlined, still easy to see */
  .commit-btn:disabled { background: transparent; border: var(--border-width) solid var(--accent); color: var(--accent); opacity: .7; }
</style>
