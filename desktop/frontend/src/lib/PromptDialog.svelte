<script lang="ts">
  import { t } from "./i18n.svelte";
  import Modal from "./Modal.svelte";
  import type { Snippet } from "svelte";

  // Asks for a name or a description before doing something (children:
  // options under the field).
  let { title, text, label, value = $bindable(), placeholder = "", confirm, busy = false, onconfirm, onclose, children }: {
    title: string; text: string; label: string; value: string; placeholder?: string; confirm: string;
    busy?: boolean; onconfirm: () => void; onclose: () => void; children?: Snippet;
  } = $props();
  const id = `prompt-${Math.random().toString(36).slice(2)}`;
</script>

<Modal {title} {onclose}>
  <p class="muted">{text}</p>
  <label for={id}>{label}</label>
  <input {id} bind:value {placeholder} />
  {@render children?.()}
  {#snippet footer()}
    <button onclick={onclose}>{t("Cancel")}</button>
    <button class="primary" disabled={!value?.trim() || busy} onclick={onconfirm}>{confirm}</button>
  {/snippet}
</Modal>
