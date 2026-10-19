<script lang="ts">
  import { t } from "./i18n.svelte";
  import Modal from "$shared/Modal.svelte";

  // A folder R3V finds nothing of a kind it knows in (only documents, a new
  // project): the person says how to track it. code: text merged line by
  // line, the folder's .gitignore followed; none: every file as it
  // is. Written in its .r3v.yaml, which can be changed later.
  let { folder, code, busy = false, onpick, onclose }: {
    folder: string;
    code: boolean; // the code kind is in this build
    busy?: boolean;
    onpick: (preset: string) => void;
    onclose: () => void;
  } = $props();
  // svelte-ignore state_referenced_locally
  let kind = $state(code ? "code" : "none");
  const name = $derived(folder.split(/[\\/]/).filter(Boolean).pop() ?? folder);
</script>

<Modal title={t("How should R3V keep “{name}”?", { name })} width={520} {onclose}>
  <p class="muted">{t("R3V doesn't recognise what this folder is (no Live set, game engine project or code project in it). Say how to keep it: you can change it later in the project's rules.")}</p>
  <div class="kinds" role="radiogroup">
    {#if code}
      <label class="kind" class:on={kind === "code"}>
        <input type="radio" name="kind" value="code" bind:group={kind} />
        <span><strong>{t("Code and documents")}</strong>
          <span class="hint">{t("Text files (code, notes, settings) are merged line by line when two people change them; a .gitignore is followed.")}</span></span>
      </label>
    {/if}
    <label class="kind" class:on={kind === "none"}>
      <input type="radio" name="kind" value="none" bind:group={kind} />
      <span><strong>{t("Files as they are")}</strong>
        <span class="hint">{t("Every file is kept as it is; when two people change the same file, you choose whose to keep.")}</span></span>
    </label>
  </div>
  {#snippet footer()}
    <button onclick={onclose}>{t("Cancel")}</button>
    <button class="primary" onclick={() => onpick(kind)} disabled={busy}>{t("Add project")}</button>
  {/snippet}
</Modal>

<style>
  .kinds { display: flex; flex-direction: column; gap: var(--sp-8); margin-top: var(--sp-12); }
  .kind { display: flex; align-items: flex-start; gap: var(--sp-10); padding: var(--sp-10) var(--sp-12); border: var(--border-width) solid var(--line);
    border-radius: var(--radius); cursor: pointer; }
  .kind.on { border-color: var(--accent); }
  .kind input { margin-top: 3px; }
  .kind span { display: flex; flex-direction: column; gap: var(--sp-2); }
  .hint { color: var(--faint); font-size: var(--fs-sm); }
</style>
