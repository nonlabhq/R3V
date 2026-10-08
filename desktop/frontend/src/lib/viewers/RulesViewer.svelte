<script lang="ts">
  import { t } from "../i18n.svelte";
  import Tx from "../Tx.svelte";
  import { api, errorText } from "../api";
  import type { RulesState } from "../../../bindings/github.com/nonlabhq/r3v/desktop/models";
  import RulesEditor from "../RulesEditor.svelte";
  import { editRulesText, folderName, leftOutText, presetName, rulesChanges } from "../rules";
  import { rulesLook, setRulesPane } from "./settings.svelte";
  import { fileLocksSet, lockingLines } from "../lockKinds";
  import TextViewer from "./TextViewer.svelte";
  import type { Side, ViewerProps } from "./types";

  // The project's own rules (.r3v.yaml): what they say in plain words, the
  // way the Rules window groups them; comparing, what changed rule by rule.
  // The file as it is now, in the Files tab, is changed right here with the
  // Rules window's editing. Both sides are read in Go (RulesAt), by the
  // parser R3V follows; a file it can't read is shown as text.
  let { root, file, a, b, compare, stamp, editable = false }: ViewerProps = $props();

  let comparing = $derived(compare && !!(a || b));
  let canEdit = $derived(editable && !compare && a?.version === "");

  let now = $state<RulesState | null>(null);    // a's rules (null: no file)
  let before = $state<RulesState | null>(null); // b's, when comparing
  let loaded = $state(false);
  let failed = $state("");
  let reloads = $state(0); // the editor changed the file
  let seq = 0, shownKey = "";
  $effect(() => {
    if (rulesLook.pane === "text") return;
    const n = ++seq, sa = a, sb = comparing ? b : null;
    stamp; reloads; // read again with them
    // Other sides: start over; the same ones read again stay shown meanwhile.
    const key = `${sa?.path}|${sa?.version}|${sb?.path}|${sb?.version}`;
    if (key !== shownKey) { shownKey = key; loaded = false; now = before = null; }
    failed = "";
    const read = (s: Side | null) => (s ? api.RulesAt(root, s.path, s.version) : Promise.resolve(null));
    Promise.all([read(sa), read(sb)])
      .then(([x, y]) => { if (n === seq) { now = x; before = y; loaded = true; } })
      .catch((e) => { if (n === seq) failed = errorText(e); });
  });

  let unreadable = $derived(now?.error ? { side: a, error: now.error } : before?.error ? { side: b, error: before.error } : null);
  let changes = $derived(comparing && loaded && !unreadable ? rulesChanges(now, before) : []);
  const sign = { add: "+", del: "−", mod: "~" } as const;
</script>

{#snippet summary(r: RulesState)}
  <section>
    <h3>{t("Tools in this project")}</h3>
    {#each r.presets as e (e.folder)}
      {@const opt = r.options.find((o) => o.name === e.preset)}
      <div class="tool">
        <strong>{folderName(e.folder)}</strong>
        <span>{presetName(e.preset)}</span>
        {#if e.found}<span class="chip found" title={t("R3V wrote this line from what it found")}>{t("found by R3V")}</span>{/if}
        {#if opt?.leftOut.length}<span class="faint">· {t("leaves out {what}", { what: leftOutText(opt.leftOut) })}</span>{/if}
      </div>
    {:else}
      <p class="faint">{t("None named: R3V finds the tools from the folders.")}</p>
    {/each}
  </section>
  <section>
    <h3>{t("Your rules")}</h3>
    {#each r.rules as ru, i (i)}
      <div class="rule">
        <span class="kind" class:keep={ru.kind === "track"}>{ru.kind === "ignore" ? t("Leave out") : t("Keep")}</span>
        <span class="mono">{ru.pattern}</span>
      </div>
    {:else}
      <p class="faint">{t("None: only the presets decide.")}</p>
    {/each}
    {#if r.rules.length > 1}<p class="faint small">{t("Later rules win.")}</p>{/if}
  </section>
  {@render other(r)}
{/snippet}

{#snippet other(r: RulesState)}
  {#if r.requires || r.gitignore}
    <section>
      <h3>{t("Other settings")}</h3>
      <ul class="settings">
        {#if r.requires}<li>{t("Needs R3V {version} or newer", { version: r.requires })}</li>{/if}
        {#if r.gitignore}<li>{t("Follows the project's .gitignore files")}</li>{/if}
      </ul>
    </section>
  {/if}
  {@const locking = lockingLines(r.teamLocks, r.fileLocks)}
  {#if locking.length}
    <section>
      <h3>{t("File locks")}</h3>
      <ul class="settings locking">
        {#each locking as line, i (i)}<li>{line}</li>{/each}
      </ul>
    </section>
  {/if}
{/snippet}

<div class="rulesview">
  <div class="bar">
    <div class="modes" title={t("The rules in plain words, or the file's text")}>
      <button class:on={rulesLook.pane === "rules"} onclick={() => setRulesPane("rules")}>{t("Rules")}</button>
      <button class:on={rulesLook.pane === "text"} onclick={() => setRulesPane("text")}>{t("Text")}</button>
    </div>
  </div>

  {#if rulesLook.pane === "text"}
    <TextViewer {root} {file} {a} {b} {compare} {stamp} />
  {:else if failed}
    <p class="muted">{t("Couldn't read it:")} {failed}</p>
  {:else if !loaded}
    <p class="faint">{t("Reading the rules…")}</p>
  {:else if unreadable}
    <div class="error-box">
      <div>⚠ {t("R3V can't read these rules ({where}): {error}", { where: unreadable.side?.label ?? "", error: unreadable.error })}</div>
      {#if canEdit}<button onclick={() => editRulesText(root)}>{t("Fix .r3v.yaml")}</button>{/if}
    </div>
    <TextViewer {root} {file} {a} {b} {compare} {stamp} />
  {:else if comparing}
    {#if changes.length}
      <ul class="changes">
        {#each changes as c, i (i)}
          <li class={c.kind}><span class="sign" aria-hidden="true">{sign[c.kind]}</span>{c.text}</li>
        {/each}
      </ul>
      {#if now && now.teamLocks && !now.teamLocks.on && (fileLocksSet(now.fileLocks) || fileLocksSet(before?.fileLocks))}
        <p class="faint small">{t("The team hasn't turned file locking on: this file's lock settings do nothing until it does.")}</p>
      {/if}
    {:else}
      <p class="muted">{t("The rules are the same: only comments or layout changed.")}</p>
    {/if}
  {:else if !now?.exists}
    <p class="muted">{t("No rules file here.")}</p>
  {:else if canEdit}
    <p class="hint"><Tx text={t("Which files go into versions. Saved in the project's {file} right away; commit it to share the rules with the team. Files left out stay on everyone's disk.")} code={{ file: ".r3v.yaml" }} /></p>
    <RulesEditor {root} onchanged={() => reloads++} />
    {@render other(now)}
  {:else}
    {@render summary(now)}
  {/if}
</div>

<style>
  .rulesview { display: flex; flex-direction: column; gap: var(--sp-8); }
  .bar { display: flex; align-items: center; gap: var(--sp-12); flex-wrap: wrap; }
  .modes { display: flex; }
  .modes button { padding: var(--sp-4) var(--sp-10); font-size: var(--fs-sm); border-radius: 0; }
  .modes button:first-child { border-radius: var(--radius) 0 0 var(--radius); }
  .modes button:last-child { border-radius: 0 var(--radius) var(--radius) 0; margin-left: -1px; }
  .modes button.on { background: var(--accent); color: var(--accent-ink); border-color: var(--accent); }
  section + section { margin-top: var(--sp-10); }
  h3 { margin: 0 0 var(--sp-6); font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--muted); }
  .hint { margin: 0; color: var(--muted); font-size: var(--fs-md); }
  .small { font-size: var(--fs-sm); }
  .mono { font-family: ui-monospace, monospace; font-size: var(--fs-sm); }
  .tool { display: flex; align-items: center; flex-wrap: wrap; gap: var(--sp-6); padding: var(--sp-6) var(--sp-10); margin-bottom: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); background: var(--panel-2); }
  .chip { font-size: var(--fs-xs); padding: 1px var(--sp-6); border-radius: var(--radius-pill); background: var(--line); color: var(--muted); white-space: nowrap; }
  .chip.found { background: var(--accent-bg); color: var(--accent); }
  .rule { display: flex; align-items: center; gap: var(--sp-10); padding: var(--sp-4) 0; }
  .kind { font-size: var(--fs-xs); padding: 1px var(--sp-8); border-radius: var(--radius-pill); background: var(--danger-bg); color: var(--danger-text); }
  .kind.keep { background: var(--accent-bg); color: var(--accent); }
  .settings { margin: 0; padding-left: var(--sp-18); font-size: var(--fs-md); }
  .error-box { display: flex; align-items: center; gap: var(--sp-10); padding: var(--sp-10) var(--sp-12); border-radius: var(--radius-lg);
    background: var(--danger-bg); border: var(--border-width) solid var(--danger-line); color: var(--danger-text); }
  .error-box > div { flex: 1; min-width: 0; }
  .changes { list-style: none; margin: 0; padding: var(--sp-8) var(--sp-10); background: var(--bg); border: var(--border-width) solid var(--line);
    border-radius: var(--radius); line-height: 1.6; user-select: text; font-size: var(--fs-md); }
  .changes .sign { display: inline-block; width: 1.2em; font-family: ui-monospace, monospace; }
  .changes .add { color: var(--add); }
  .changes .del { color: var(--del); }
  .changes .mod { color: var(--mod); }
</style>
