<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, ago, errorText, type Branch, type Version, type Workspace } from "./api";
  import Modal from "./Modal.svelte";
  import { toast } from "./notify.svelte";
  import { branchGraph } from "./branchGraph";
  import { MAIN, branchLabel } from "./branches";

  // Archiving a branch (docs/design/branch-tree.md): it goes with the
  // branches made from it, each listed with who is on it and what of theirs
  // isn't merged anywhere. Ticked by default, but for one with someone's
  // work on it (or the one you're on); one left out stays, coming from the
  // nearest branch above it that stays. Archived branches come back from
  // the project's settings.
  let { root, branch, branches, history, current, merged = false, onarchived, onclose }: {
    root: string;
    branch: string; // its key
    branches: Branch[];
    history: Version[];
    current: string; // the branch you are on
    merged?: boolean; // just merged (worded so)
    onarchived: () => void;
    onclose: () => void;
  } = $props();

  // The subtree, parents first, each with its depth.
  let tree = $derived.by(() => {
    const g = branchGraph(history, branches.map((b) => ({ name: b.name, latest: b.latest?.id ?? "", parent: b.parent })), current);
    const parentOf = new Map<string, string>();
    for (const c of g.chains) if (c.name && c.parent?.name) parentOf.set(c.name, c.parent.name);
    for (const b of branches) if (b.parent && !parentOf.has(b.name)) parentOf.set(b.name, b.parent); // (no history at hand)
    const out: { key: string; depth: number; parent: string }[] = [];
    const walk = (key: string, depth: number, parent: string) => {
      out.push({ key, depth, parent });
      for (const b of branches) if (b.name !== MAIN && parentOf.get(b.name) === key && !out.some((o) => o.key === b.name)) walk(b.name, depth + 1, key);
    };
    walk(branch, 0, parentOf.get(branch) ?? MAIN);
    return out;
  });

  let people = $state<Workspace[] | null>(null);
  let only = $state<Record<string, number>>({});
  $effect(() => {
    api.Workspaces(root).then((w) => (people = w ?? [])).catch(() => (people = []));
    for (const n of tree) api.VersionsOnlyOnBranch(root, n.key).then((c) => (only = { ...only, [n.key]: c })).catch(() => {});
  });
  // Others' work on a branch: on it (with what), or changes parked there.
  const workOn = (key: string) => (people ?? []).filter((w) => w.branch === key || w.parked.includes(key));
  let ticked = $state<Record<string, boolean>>({});
  let decided = $state(false);
  $effect(() => {
    if (decided || people === null) return;
    const out: Record<string, boolean> = {};
    for (const n of tree) out[n.key] = n.key === branch || (n.key !== current && workOn(n.key).length === 0);
    ticked = out;
    decided = true;
  });
  const blocked = (key: string) => key === current;
  function what(w: Workspace, key: string): string {
    if (w.branch !== key) return t("{name} has changes parked on it ({when})", { name: w.member, when: ago(w.time) });
    const bits = [
      w.changes ? tn(w.changes, "{n} change not committed", "{n} changes not committed") : "",
      w.unshared ? tn(w.unshared, "{n} version not shared", "{n} versions not shared") : "",
    ].filter(Boolean);
    return bits.length ? t("{name} is on it: {work} ({when})", { name: w.member, work: bits.join(", "), when: ago(w.time) })
      : t("{name} is on it ({when})", { name: w.member, when: ago(w.time) });
  }

  let busy = $state(false);
  async function archive() {
    busy = true;
    const keys = tree.filter((n) => ticked[n.key]).map((n) => n.key);
    // Those left out: from the nearest branch above them that stays.
    const keep: Record<string, string> = {};
    for (const n of tree) {
      if (ticked[n.key]) continue;
      let p = n.parent;
      while (ticked[p]) p = tree.find((x) => x.key === p)?.parent ?? MAIN;
      if (p !== n.parent) keep[n.key] = p;
    }
    try {
      await api.ArchiveBranches(root, keys, keep);
      toast(tn(keys.length, "Archived {n} branch. It can come back from the project's settings.",
        "Archived {n} branches. They can come back from the project's settings.") , "ok", 8000);
      onarchived();
      onclose();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      busy = false;
    }
  }
  const label = (key: string) => `“${branchLabel(branches, key)}”`;
</script>

<Modal title={merged ? t("Archive {branch} now that it's merged?", { branch: label(branch) }) : t("Archive {branch}?", { branch: label(branch) })} width={520} {onclose}>
  <p class="muted">{tree.length > 1
    ? t("It goes with the branches made from it. Untick one to keep it. Nothing is lost: versions stay, and archived branches come back from the project's settings.")
    : t("Nothing is lost: its versions stay, and it comes back from the project's settings.")}</p>
  <ul>
    {#each tree as n (n.key)}
      <li style:padding-left="{n.depth * 20}px">
        <label>
          <input type="checkbox" bind:checked={() => !!ticked[n.key], (v) => (ticked = { ...ticked, [n.key]: v })}
            disabled={n.key === branch || blocked(n.key)} />
          <span class="name">{label(n.key)}</span>
        </label>
        <div class="info">
          {#if blocked(n.key)}<span class="warn">{t("You're on it: it stays")}</span>{/if}
          {#each workOn(n.key) as w (w.memberId + w.time)}<span class="warn">{what(w, n.key)}</span>{/each}
          {#if only[n.key]}<span>{tn(only[n.key], "{n} version on no other branch", "{n} versions on no other branch")}</span>{/if}
        </div>
      </li>
    {/each}
  </ul>
  {#snippet footer()}
    <button onclick={onclose}>{merged ? t("Keep it") : t("Cancel")}</button>
    <button class="primary" onclick={archive} disabled={busy || people === null || blocked(branch)}>{t("Archive")}</button>
  {/snippet}
</Modal>

<style>
  ul { list-style: none; margin: var(--sp-12) 0 0; padding: 0; display: flex; flex-direction: column; gap: var(--sp-8); }
  label { display: flex; align-items: center; gap: var(--sp-8); }
  .name { font-weight: var(--fw-semibold); }
  .info { display: flex; flex-direction: column; gap: var(--sp-2); margin-left: 26px; font-size: var(--fs-sm); color: var(--faint); }
  .warn { color: var(--warn); }
</style>
