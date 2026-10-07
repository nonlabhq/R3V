<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText } from "./api";
  import Modal from "./Modal.svelte";
  import { toast } from "./notify.svelte";

  // A milestone: a version given a name for the whole team (Sent to the
  // label, Final mix…), with a note. New, or one to rename or take away
  // (the version stays).
  let { root, version, id = "", name: start = "", note: startNote = "", label, onchanged, onclose }: {
    root: string;
    version: string;
    id?: string;    // an existing milestone ("" for a new one)
    name?: string;
    note?: string;
    label: string;  // the version, for the title
    onchanged: () => void;
    onclose: () => void;
  } = $props();

  // svelte-ignore state_referenced_locally
  let name = $state(start);
  // svelte-ignore state_referenced_locally
  let note = $state(startNote);
  let busy = $state(false);

  async function act(call: () => Promise<unknown>, done: string) {
    busy = true;
    try {
      await call();
      toast(done, "ok");
      onchanged();
      onclose();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      busy = false;
    }
  }
  const save = () => name.trim() && act(
    () => (id ? api.EditMilestone(root, id, name, note) : api.AddMilestone(root, version, name, note)),
    id ? t("Milestone changed for everyone in the team") : t("Milestone added for everyone in the team"));
  const remove = () => act(() => api.RemoveMilestone(root, id), t("Milestone taken away; the version stays"));
</script>

<Modal title={id ? t("Milestone") : t("Mark as a milestone")} width={460} {onclose}>
  <p class="muted">{t("Gives {version} a name the whole team sees on the history.", { version: label })}</p>
  <label class="field">
    <span>{t("Name")}</span>
    <!-- svelte-ignore a11y_autofocus -->
    <input bind:value={name} maxlength="64" placeholder={t("Sent to the label, v1")} autofocus
      onkeydown={(e) => { if (e.key === "Enter") save(); }} />
  </label>
  <label class="field">
    <span>{t("Note")}</span>
    <textarea bind:value={note} maxlength="1000" rows="3" placeholder={t("What it is, for whom (optional)")}></textarea>
  </label>

  {#snippet footer()}
    {#if id}<button class="danger-btn" onclick={remove} disabled={busy}>{t("Take away")}</button>{/if}
    <span class="grow"></span>
    <button onclick={onclose}>{t("Cancel")}</button>
    <button class="primary" onclick={save} disabled={busy || !name.trim()}>{id ? t("Save") : t("Add milestone")}</button>
  {/snippet}
</Modal>

<style>
  .muted { color: var(--muted); margin: 0 0 var(--sp-12); }
  .field { display: flex; flex-direction: column; gap: var(--sp-4); margin-bottom: var(--sp-10); }
  .field span { font-size: var(--fs-sm); color: var(--faint); }
  textarea { resize: vertical; font: inherit; }
  .grow { flex: 1; }
  .danger-btn { color: var(--danger); }
</style>
