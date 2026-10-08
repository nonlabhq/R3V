<script lang="ts">
  import { t } from "./i18n.svelte";
  import Tx from "./Tx.svelte";
  import { api } from "./api";
  import Modal from "./Modal.svelte";
  import RulesEditor from "./RulesEditor.svelte";
  import { editRulesText } from "./rules";

  // A project's rules (.r3v.yaml) without writing YAML, in a window (the
  // editing itself is RulesEditor's, shared with the rules file's viewer).
  let { root, onclose }: { root: string; onclose: () => void } = $props();
</script>

<Modal title={t("Rules")} {onclose} width={860}>
  <p class="hint"><Tx text={t("Which files go into versions. Saved in the project's {file} right away; commit it to share the rules with the team. Files left out stay on everyone's disk.")} code={{ file: ".r3v.yaml" }} /></p>

  <RulesEditor {root} />

  {#snippet footer()}
    <button class="ghost" onclick={() => api.OpenURL("https://github.com/nonlabhq/R3V/blob/main/docs/profiles.md")}>{t("Guide ↗")}</button>
    <button onclick={() => editRulesText(root)}>{t("Edit as text")}</button>
    <button class="primary" onclick={onclose}>{t("Done")}</button>
  {/snippet}
</Modal>

<style>
  .hint { margin: 0 0 var(--sp-8); color: var(--muted); font-size: var(--fs-md); }
</style>
