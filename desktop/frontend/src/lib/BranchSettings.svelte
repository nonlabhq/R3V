<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type Branch } from "./api";
  import Modal from "./Modal.svelte";
  import Swatches from "./Swatches.svelte";
  import { toast } from "./notify.svelte";
  import { MAIN, branchColor, branchLabel } from "./branches";
  import { cssColor } from "./palette";

  // A branch's settings, for the whole team: its name (any words, any
  // language) and its colour. Renaming moves nothing: whoever is on it
  // keeps working.
  let { root, branch, branches, onchanged, onclose }: {
    root: string;
    branch: string; // its key
    branches: Branch[];
    onchanged: () => void;
    onclose: () => void;
  } = $props();

  // svelte-ignore state_referenced_locally
  let name = $state(branchLabel(branches, branch));
  // svelte-ignore state_referenced_locally
  let color = $state(branchColor(branches, branch));
  let saved = $derived(branchLabel(branches, branch));
  let busy = $state(false);
  const isMain = $derived(branch === MAIN);

  async function save(nextName: string, nextColor: string) {
    if (busy) return;
    busy = true;
    try {
      await api.SetBranchRecord(root, branch, nextName.trim(), nextColor);
      color = nextColor;
      onchanged();
      return true;
    } catch (e) {
      toast(errorText(e), "error");
      return false;
    } finally {
      busy = false;
    }
  }
  async function rename() {
    if (await save(name, isMain ? "" : color)) toast(t("Renamed for everyone in the team"), "ok");
  }
</script>

<Modal title={t("Branch settings")} width={460} {onclose}>
  <section>
    <h3>{t("Name")}</h3>
    <div class="line">
      <span class="dot" style:--c={isMain ? "var(--lane-0)" : cssColor(color)} aria-hidden="true"></span>
      <input bind:value={name} maxlength="64" aria-label={t("Branch name")}
        onkeydown={(e) => { if (e.key === "Enter" && name.trim() && name.trim() !== saved) rename(); }} />
      <button onclick={rename} disabled={busy || !name.trim() || name.trim() === saved}>{t("Rename")}</button>
    </div>
    <p class="hint">{t("Any words, in any language. Renaming moves nothing: whoever is on it keeps working.")}</p>
  </section>
  <section>
    <h3>{t("Colour")}</h3>
    {#if isMain}
      <p class="hint">{t("The main branch keeps R3V's orange.")}</p>
    {:else}
      <Swatches value={color} label={t("Colour")} disabled={busy} onpick={(c) => save(saved, c)} />
      <p class="hint">{t("The whole team sees it, on the history and in the menus.")}</p>
    {/if}
  </section>

  {#snippet footer()}
    <button class="primary" onclick={onclose}>{t("Done")}</button>
  {/snippet}
</Modal>

<style>
  section { padding: var(--sp-12) 0; border-top: var(--border-width) solid var(--line); }
  section:first-child { border-top: none; padding-top: 0; }
  h3 { margin: 0 0 var(--sp-8); font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); font-weight: var(--fw-semibold); }
  .line { display: flex; gap: var(--sp-8); align-items: center; }
  .line input { flex: 1; }
  .dot { flex: none; width: 12px; height: 12px; border-radius: 50%; background: var(--c); }
  .hint { color: var(--faint); font-size: var(--fs-sm); margin: var(--sp-6) 0 0; }
</style>
