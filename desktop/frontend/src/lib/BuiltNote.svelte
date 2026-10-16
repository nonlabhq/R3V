<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, errorText, formatBytes, type State } from "./api";
  import { toast } from "./notify.svelte";

  // Over the commit box: a new folder that looks like a build's output
  // (target-dx12/ beside the target/ the rules leave out). Committed, it
  // would stay in the team's storage for good, so it's said before; a
  // word, not a refusal. "Leave it out" adds the rule to .r3v.yaml.
  let { root, st, onrules }: {
    root: string;
    st: State;
    onrules?: () => void; // .r3v.yaml changed: the changes are read again
  } = $props();

  let adding = $state("");
  async function leaveOut(rule: string) {
    adding = rule;
    try {
      await api.AddIgnoreRule(root, rule);
      toast(t("Left out of versions: {pattern} — a rule in .r3v.yaml; commit it to share it with the team", { pattern: rule }), "ok", 7000);
      onrules?.();
    } catch (e) {
      toast(errorText(e), "error", 9000);
    } finally {
      adding = "";
    }
  }
</script>

{#each st.built ?? [] as b (b.folder)}
  {@const vars = { folder: b.folder + "/", like: b.like, files: tn(b.files, "{n} file", "{n} files"), size: formatBytes(b.bytes) }}
  <!-- (said by kind: a Live set's Backup-2024/ is no build's output) -->
  <div class="built" role="note">
    <p>{b.kind === "name"
      ? t("“{folder}” is named like “{like}”, which the rules leave out ({files}, {size}). Committed, it stays in the team's storage for good.", vars)
      : b.kind === "size"
        ? t("“{folder}” is big ({size}) and new ({files}). Committed, it stays in the team's storage for good.", vars)
        : t("“{folder}” looks like a build's output ({files}, {size}). Committed, it stays in the team's storage for good.", vars)}</p>
    <button disabled={!!adding} onclick={() => leaveOut(b.rule)}
      title={t("Adds the rule ignore “{rule}” to .r3v.yaml", { rule: b.rule })}>{t("Leave it out")}</button>
  </div>
{/each}

<style>
  .built { display: flex; align-items: center; gap: var(--sp-10); margin-bottom: var(--sp-8); padding: var(--sp-6) var(--sp-10);
    border-radius: var(--radius); background: var(--warn-bg); box-shadow: inset 0 0 0 var(--border-width) var(--warn-line);
    color: var(--warn-text); font-size: var(--fs-sm); }
  .built p { flex: 1; margin: 0; line-height: 1.4; }
  .built button { flex: none; white-space: nowrap; }
</style>
