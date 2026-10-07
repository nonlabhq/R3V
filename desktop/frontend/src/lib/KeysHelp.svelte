<script lang="ts">
  import { t } from "./i18n.svelte";
  import Modal from "./Modal.svelte";

  // The keyboard shortcuts, listed (Ctrl+/).
  let { onclose }: { onclose: () => void } = $props();

  const groups = () => [
    { title: t("Window and tabs"), keys: [
      ["Ctrl+T", t("New tab")],
      ["Ctrl+W", t("Close the tab")],
      ["Ctrl+Shift+T", t("Open the tab closed last again")],
      ["Ctrl+Tab · Ctrl+Shift+Tab", t("Next and previous tab")],
      ["Ctrl+1 … 8 · Ctrl+9", t("A tab, the last tab")],
      ["Ctrl+\\", t("Show or hide the sidebar")],
      ["F5 · Ctrl+R", t("Read the project again")],
    ] },
    { title: t("Lists of files and changes"), keys: [
      ["↑ ↓ · Home End", t("Pick the next, previous, first or last")],
      ["→ ←", t("Open or close a folder; in the Files tab, go into it or up")],
      ["Enter · Backspace", t("Open a folder · go up (Files tab)")],
      ["Space", t("Play or pause the audio shown")],
    ] },
    { title: t("Overview"), keys: [
      ["↑ ↓", t("The next or previous version in the graph")],
      ["Ctrl+scroll", t("Spread the graph out or pack it in")],
    ] },
    { title: t("Everywhere"), keys: [
      ["Ctrl+Enter", t("Commit (in the commit box)")],
      ["Esc", t("Close a dialog or a menu")],
      ["Ctrl+/", t("This list")],
    ] },
  ];
</script>

<Modal title={t("Keyboard shortcuts")} {onclose} width={560}>
  {#each groups() as g (g.title)}
    <h3>{g.title}</h3>
    <dl>
      {#each g.keys as [k, what] (k)}
        <dt><kbd>{k}</kbd></dt><dd>{what}</dd>
      {/each}
    </dl>
  {/each}
</Modal>

<style>
  h3 { margin: var(--sp-12) 0 var(--sp-6); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); }
  h3:first-child { margin-top: 0; }
  dl { display: grid; grid-template-columns: 200px 1fr; gap: var(--sp-6) var(--sp-16); margin: 0; }
  dt { margin: 0; }
  dd { margin: 0; color: var(--muted); }
  kbd { font-family: var(--font-mono); font-size: var(--fs-sm); padding: 1px var(--sp-6); border: var(--border-width) solid var(--line);
    border-radius: var(--radius-sm); background: var(--panel-2); white-space: nowrap; }
</style>
