<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, errorText, type Branch } from "./api";
  import Modal from "./Modal.svelte";
  import Swatches from "./Swatches.svelte";
  import { toast } from "./notify.svelte";
  import { MAIN, branchColor, branchLabel } from "./branches";
  import { cssColor } from "./palette";
  import ConfirmDialog from "./ConfirmDialog.svelte";

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
  // Deleting: the versions stay, and the branch can come back from the
  // project's settings (Deleted branches).
  const isCurrent = $derived(branches.find((b) => b.name === branch)?.current ?? false);
  let deleting = $state<{ only: number } | null>(null);
  async function askDelete() {
    busy = true;
    try {
      deleting = { only: await api.VersionsOnlyOnBranch(root, branch) };
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      busy = false;
    }
  }
  async function del() {
    busy = true;
    try {
      await api.DeleteBranch(root, branch);
      toast(t("Deleted “{branch}”. It can come back from the project's settings.", { branch: saved }), "ok", 8000);
      deleting = null;
      onchanged();
      onclose();
    } catch (e) {
      toast(errorText(e), "error");
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

  {#if !isMain}
    <section>
      <h3 class="danger">{t("Delete")}</h3>
      {#if isCurrent}
        <p class="hint">{t("You're on this branch: switch to another one to delete it.")}</p>
      {:else}
        <div class="line">
          <p class="hint grow">{t("Takes it away for the whole team. Its versions stay, and it can come back from the project's settings.")}</p>
          <button class="danger-btn" onclick={askDelete} disabled={busy}>{t("Delete branch…")}</button>
        </div>
      {/if}
    </section>
  {/if}

  {#snippet footer()}
    <button class="primary" onclick={onclose}>{t("Done")}</button>
  {/snippet}
</Modal>

{#if deleting}
  <ConfirmDialog title={t("Delete “{branch}”?", { branch: saved })} danger confirm={t("Delete branch")}
    text={deleting.only
      ? tn(deleting.only, "{n} version is only on this branch. It stays in the team's storage, and the branch can come back from the project's settings.",
        "{n} versions are only on this branch. They stay in the team's storage, and the branch can come back from the project's settings.")
      : t("Every version on it is on another branch too. The branch can come back from the project's settings.")}
    onconfirm={del} onclose={() => (deleting = null)} />
{/if}

<style>
  section { padding: var(--sp-12) 0; border-top: var(--border-width) solid var(--line); }
  section:first-child { border-top: none; padding-top: 0; }
  h3 { margin: 0 0 var(--sp-8); font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); font-weight: var(--fw-semibold); }
  .line { display: flex; gap: var(--sp-8); align-items: center; }
  .line input { flex: 1; }
  .dot { flex: none; width: 12px; height: 12px; border-radius: 50%; background: var(--c); }
  .hint { color: var(--faint); font-size: var(--fs-sm); margin: var(--sp-6) 0 0; }
  .grow { flex: 1; margin: 0; }
  h3.danger { color: var(--danger); }
  .danger-btn { color: var(--danger); flex: none; }
</style>
