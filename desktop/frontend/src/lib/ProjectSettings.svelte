<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, errorText, type ProjectInfo, type Progress, type TeamProject, type TeamSummary } from "./api";
  import Modal from "$shared/Modal.svelte";
  import RulesWindow from "./RulesWindow.svelte";
  import Tx from "./Tx.svelte";
  import { toast } from "./notify.svelte";
  import ProjectIcon from "$shared/ProjectIcon.svelte";
  import LookPicker from "$shared/LookPicker.svelte";
  import { portal } from "$shared/portal";
  import BranchSettings from "./BranchSettings.svelte";
  import ConfirmDialog from "$shared/ConfirmDialog.svelte";
  import ProjectCheck from "./ProjectCheck.svelte";
  import MoveProjectDialog from "./MoveProjectDialog.svelte";
  import ProjectWorkflow from "./ProjectWorkflow.svelte";
  import SettingsTabs from "$shared/team/SettingsTabs.svelte";
  import { branchLabel, branchLane } from "$shared/branches";
  import { ago, type BranchList, type ArchivedBranch, type StepsInfo } from "./api";

  // A project's settings: its name, where it is, its branches, its rules,
  // its workflow, and what can be done with it (check it, unlink or delete
  // it). In the project's Settings tab, in tabs down the left as a team's
  // settings are. The actions that need a confirmation of their own are the
  // caller's.
  let { p, team, teams = [], progress = null, inline = false, onclose, onrenamed, oncheck, ondelete, onunlink, onlocate, onmoved }: {
    p: TeamProject;
    inline?: boolean;       // in the project's Settings tab, not a dialog
    team?: TeamSummary;     // the project's team (none: on this computer only)
    onclose: () => void;
    onrenamed: () => void;
    oncheck: () => void;
    ondelete: () => void;
    teams?: TeamSummary[];     // the teams on this computer (to move it to)
    progress?: Progress | null; // the project's step under way (a move)
    onmoved?: (teamId: string, copied: boolean) => void;
    onunlink: () => void;
    onlocate: () => void;
  } = $props();

  const here = $derived(p.status === "downloaded");
  let healthOpen = $state(false); // the project check: Live version, samples, plugins
  let info = $state<ProjectInfo | null>(null);
  $effect(() => {
    if (!here) return;
    api.ProjectInfo(p.root).then((i) => (info = i)).catch(() => {});
  });

  // Agents' steps (Nightly): this computer's choices.
  let steps = $state<StepsInfo | null>(null);
  $effect(() => {
    if (!here) return;
    api.Steps(p.root, "").then((s) => (steps = s)).catch(() => {});
  });
  // Claude Code in this folder (not a workspace R3V made) makes steps once
  // R3V's hooks are in its settings.
  let hooking = $state(false);
  async function setUpSteps() {
    hooking = true;
    try {
      await api.SetUpSteps(p.root);
      steps = await api.Steps(p.root, "");
      toast(t("Set up: Claude Code makes steps here from its next prompt (start it again if it's running)."), "ok", 9000);
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      hooking = false;
    }
  }
  // svelte-ignore state_referenced_locally
  let name = $state(p.name); // the field starts with the name
  let renaming = $state(false);
  async function rename() {
    renaming = true;
    try {
      await api.RenameProject(team?.id ?? "", p.id, here ? p.root : "", name.trim());
      toast(team ? t("Renamed for everyone in {team}", { team: team.name }) : t("Renamed"), "ok");
      onrenamed();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      renaming = false;
    }
  }

  // Its icon, colour and picture, for the whole team (Nightly: a team that
  // keeps looks): LookPicker saves each pick; the list follows.
  async function setLook(icon: string, color: string) {
    await api.SetProjectLook(team!.id, p.id, icon, color);
    onrenamed();
  }
  async function setPicture(url: string) {
    await api.SetProjectPicture(team!.id, p.id, url);
    onrenamed();
  }

  // The project's branches, each opening its settings (where the team keeps
  // branch names: Nightly).
  let branchList = $state<BranchList | null>(null);
  let branchOpen = $state<string | null>(null);
  let moving = $state(false); // the move dialog open
  let archived = $state<ArchivedBranch[]>([]);
  let archivedOpen = $state(false); // the archived ones listed (folded to their count)
  const loadBranches = () => {
    api.BranchList(p.root).then((l) => (branchList = l)).catch(() => {});
    api.ArchivedBranches(p.root).then((d) => (archived = d ?? [])).catch(() => {});
  };
  // Deleting an archived branch for good (its versions stay).
  let forgetting = $state<ArchivedBranch | null>(null);
  async function forget() {
    const d = forgetting!;
    forgetting = null;
    try {
      await api.DeleteBranch(p.root, d.name);
      toast(t("Deleted “{branch}” for good.", { branch: d.label || d.name }), "ok");
      loadBranches();
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
  let restoring = $state("");
  async function restore(d: ArchivedBranch) {
    restoring = d.name;
    try {
      await api.UnarchiveBranch(p.root, d.name);
      toast(t("“{branch}” is back, where it was.", { branch: d.label || d.name }), "ok");
      loadBranches();
      onrenamed();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      restoring = "";
    }
  }
  $effect(() => {
    if (here && team) loadBranches();
  });

  async function openRules() {
    try {
      await api.OpenRules(p.root);
      toast(t("Save the file, then R3V follows the new rules"), "info");
    } catch (e) {
      toast(errorText(e), "error");
    }
  }

  let rulesOpen = $state(false);
  let unlinkSure = $state(false);
  // The tabs, those with something in them: General (name, where it is,
  // checks, the danger zone), then Branches, Rules and Workflow.
  type Tab = "general" | "branches" | "rules" | "workflow";
  const hasBranches = $derived(!!branchList?.names && !!branchList.branches.length);
  const hasWorkflow = $derived(here && !!info && (!!steps?.available || !!team?.hosted));
  let tabs = $derived<{ id: Tab; label: string }[]>([{ id: "general", label: t("General") },
    ...(hasBranches ? [{ id: "branches" as Tab, label: t("Branches") }] : []),
    ...(here && info ? [{ id: "rules" as Tab, label: t("Rules") }] : []),
    ...(hasWorkflow ? [{ id: "workflow" as Tab, label: t("Workflow") }] : [])]);
  const presetName = (preset: string) => ({ ableton: t("Ableton Live project"), unity: t("Unity project"),
    unreal: t("Unreal project"), godot: t("Godot project"), design: t("Design files"), code: t("Code and documents"), none: t("Files as they are") } as Record<string, string>)[preset] ?? preset;
</script>


{#if rulesOpen}
  <RulesWindow root={p.root} onclose={() => { rulesOpen = false; api.ProjectInfo(p.root).then((i) => (info = i)).catch(() => {}); }} />
{:else}
{#snippet naming()}
  <section>
    <h3>{t("Name")}</h3>
    <div class="line">
      {#if team?.looks}
        <LookPicker seed={p.id} name={p.name} icon={p.icon ?? ""} color={p.color ?? ""} picture={p.picture ?? ""}
          label={t("Icon and colour")} initialLabel={t("The project's initial")} onlook={setLook} onpicture={setPicture} />
      {:else}
        <ProjectIcon p={{ id: p.id, name: p.name, status: "downloaded", icon: p.icon, color: p.color, picture: p.picture }} size={36} />
      {/if}
      <input bind:value={name} maxlength="100" aria-label={t("Project name")}
        onkeydown={(e) => { if (e.key === "Enter" && name.trim() && name.trim() !== p.name) rename(); }} />
      <button onclick={rename} disabled={renaming || !name.trim() || name.trim() === p.name}>{renaming ? t("Renaming…") : t("Rename")}</button>
    </div>
    <p class="hint">{team ? t("Project name shared by the whole team.") : t("Project name in R3V.")} {t("Local folder keeps its name.")}</p>
  </section>
{/snippet}

{#snippet place()}
  <section>
    <h3>{t("Where")}</h3>
    <dl>
      <dt>{t("Team")}</dt><dd>{team ? team.name : t("This computer only")}</dd>
      {#if p.root}
        <dt>{t("Folder")}</dt>
        <dd class="folder">
          <span class="mono path" title={p.root}>{p.root}</span>
          {#if p.status === "missing"}
            <button class="small" onclick={onlocate}>{t("Locate…")}</button>
          {:else}
            <button class="small" onclick={() => api.ShowFolder(p.root)}>{t("Open folder")}</button>
          {/if}
        </dd>
      {/if}
      {#if info}<dt>{t("Branch")}</dt><dd>{info.branch}</dd>{/if}
      {#if p.id}<dt>{t("Project ID")}</dt><dd class="mono faint">{p.id}</dd>{/if}
    </dl>
  </section>
{/snippet}

{#snippet branchesPart()}
  {#if branchList}
  <section>
    <h3>{t("Branches")}</h3>
    <ul class="branches">
      {#each branchList.branches as b (b.name)}
        <li>
          <button class="ghost brow" onclick={() => (branchOpen = b.name)} title={t("Branch settings")}>
            <span class="bdot" style:--c="var(--lane-{branchLane(branchList.branches, b.name)})"></span>
            <span class="bname">{branchLabel(branchList.branches, b.name)}</span>
            {#if b.current}<span class="here">{t("you're on it")}</span>{/if}
            <span class="faint">{[tn(b.versions, "{n} version", "{n} versions"), ...(b.latest ? [b.latest.author, ago(b.latest.time)] : [])].join(" · ")}</span>
          </button>
        </li>
      {/each}
    </ul>
    {#if archived.length}
      <button class="ghost fold" aria-expanded={archivedOpen} onclick={() => (archivedOpen = !archivedOpen)}>
        {t("Archived ({n})", { n: archived.length })}<span class="chev" aria-hidden="true">›</span></button>
      {#if archivedOpen}
      <ul class="branches" aria-label={t("Archived branches")}>
        {#each archived as d (d.name)}
          <li class="drow">
            <span class="bdot gone" style:--c="var(--lane-{branchLane([d], d.name)})"></span>
            <span class="bname">{d.label || d.name}</span>
            <span class="faint">{d.by ? t("archived by {name} {when}", { name: d.by, when: ago(d.time) }) : t("archived {when}", { when: ago(d.time) })}</span>
            <button class="small" onclick={() => restore(d)} disabled={!!restoring}>{restoring === d.name ? t("Unarchiving…") : t("Unarchive")}</button>
            <button class="small danger-text" onclick={() => (forgetting = d)} disabled={!!restoring}>{t("Delete…")}</button>
          </li>
        {/each}
      </ul>
      {/if}
    {/if}
  </section>
  {/if}
{/snippet}

{#snippet rulesPart()}
  {#if info}
  <section>
    <h3>{t("Rules")}</h3>
    <p class="hint"><Tx text={t("Which files R3V tracks, set in the project's {file}. The file is committed with the project, so everyone uses the same rules.")} code={{ file: ".r3v.yaml" }} /></p>
    <ul class="applied">
      {#each info.rules.applied as a}
        <li><strong>{presetName(a.preset)}</strong>
          <span class="faint">{a.folder ? t("in {folder}", { folder: `${a.folder}/` }) : t("the project folder")}{a.detected ? ` · ${t("detected")}` : ""}</span></li>
      {:else}
        <li class="faint">{t("Files as they are: every file is tracked.")}</li>
      {/each}
    </ul>
    {#if info.rules.error}<p class="error">⚠ {info.rules.error}</p>{/if}
    <div class="line">
      <button class="primary" onclick={() => (rulesOpen = true)}>{t("Rules…")}</button>
      <button onclick={openRules}>{t("Edit as text")}</button>
      <button class="ghost" onclick={() => api.OpenURL("https://github.com/nonlabhq/R3V/blob/main/docs/profiles.md")}>{t("Guide ↗")}</button>
    </div>
  </section>
  {/if}
{/snippet}

{#snippet workflowPart()}
  <!-- (the team's workflow, the project's own over it: file locking on R3V
       Cloud teams, agents' settings where this build has agents) -->
  {#if steps?.available || team?.hosted}<ProjectWorkflow root={p.root} />{/if}
  {#if steps?.available}
    {#if !steps.settings.off && !steps.hooked}
      <section>
        <div class="action">
          <div><strong>{t("Claude Code here doesn't make steps yet")}</strong>
            <p class="hint">{t("An agent working in this folder itself (not in a workspace) makes steps once R3V's hooks are in its settings (.claude/settings.local.json, never versioned).")}</p></div>
          <button onclick={setUpSteps} disabled={hooking}>{t("Set up")}</button>
        </div>
      </section>
    {/if}
  {/if}
{/snippet}

{#snippet healthPart()}
  <section>
    <h3>{t("Health")}</h3>
    <!-- (what a project check looks at: a Live project's; other kinds have none yet) -->
    {#if info?.tool === "Ableton Live"}
      <div class="action">
        <div><strong>{t("Project check")}</strong><p class="hint">{t("Check the project: Live version, samples, plugins")}</p></div>
        <button onclick={() => (healthOpen = true)}>{t("Check…")}</button>
      </div>
    {/if}
    <div class="action">
      <div><strong>{t("Check project…")}</strong><p class="hint">{t("Reads its whole history again, looking for damage.")}</p></div>
      <button onclick={oncheck}>{t("Check…")}</button>
    </div>
  </section>
{/snippet}

{#snippet danger()}
  <section class="danger-zone">
    <h3>{t("Danger zone")}</h3>
    {#if p.root}
      <div class="action">
        <div><strong>{t("Unlink folder")}</strong>
          <p class="hint">{team ? t("R3V stops listing this folder (the team's copy stays listed, to download).") : t("R3V stops listing this folder.")}
            {t("Nothing is deleted: the folder keeps its files and versions, and can be added again.")}</p></div>
        {#if unlinkSure}
          <button class="danger" onclick={onunlink}>{t("Unlink")}</button>
        {:else}
          <button onclick={() => (unlinkSure = true)}>{t("Unlink…")}</button>
        {/if}
      </div>
    {/if}
    {#if team && here && onmoved}
      <div class="action">
        <div><strong>{t("Move to another team…")}</strong>
          <p class="hint">{t("Its whole history goes to another team (or a copy of it). The folder here stays as it is.")}</p></div>
        <button onclick={() => (moving = true)}>{t("Move…")}</button>
      </div>
    {/if}
    {#if team}
      <div class="action">
        <div><strong>{t("Delete from {team}…", { team: team.name })}</strong>
          <p class="hint">{t("Removes it and all its versions from the team, for everyone.")}</p></div>
        <button class="danger" onclick={ondelete}>{t("Delete…")}</button>
      </div>
    {/if}
  </section>
{/snippet}

<!-- (in a dialog: everything on one page, as before the tabs) -->
{#snippet body()}
  {@render naming()}
  {#if hasBranches}{@render branchesPart()}{/if}
  {@render place()}
  {#if here && info}
    {@render rulesPart()}
    {@render workflowPart()}
    {@render healthPart()}
  {/if}
  {@render danger()}
{/snippet}
{#if inline}
  <div class="inline">
    <SettingsTabs {tabs} label={t("Project settings")} remember="r3v.projectSettingsTab">
      {#snippet content(tab)}
        {#if tab === "branches"}
          {@render branchesPart()}
        {:else if tab === "rules"}
          {@render rulesPart()}
        {:else if tab === "workflow"}
          {@render workflowPart()}
        {:else}
          {@render naming()}
          {@render place()}
          {#if here && info}{@render healthPart()}{/if}
          {@render danger()}
        {/if}
      {/snippet}
    </SettingsTabs>
  </div>
{:else}
<Modal title={t("Project settings")} {onclose} width={600}>
  {@render body()}
  {#snippet footer()}
    <button onclick={onclose}>{t("Close")}</button>
  {/snippet}
</Modal>
{/if}
{/if}

{#if healthOpen}
  <Modal title={t("Project check")} onclose={() => (healthOpen = false)}>
    <ProjectCheck root={p.root} mode="check" />
    {#snippet footer()}
      <button class="primary" onclick={() => (healthOpen = false)}>{t("OK")}</button>
    {/snippet}
  </Modal>
{/if}

{#if moving && team && onmoved}
  <MoveProjectDialog root={p.root} name={p.name} {team} {teams} {progress}
    onmoved={(id, copied) => { moving = false; onmoved(id, copied); }} onclose={() => (moving = false)} />
{/if}

{#if branchOpen !== null && branchList}
  <BranchSettings root={p.root} branch={branchOpen} branches={branchList.branches}
    onchanged={() => { loadBranches(); onrenamed(); }} onclose={() => (branchOpen = null)} />
{/if}

{#if forgetting}
  <ConfirmDialog title={t("Delete “{branch}” for good?", { branch: forgetting.label || forgetting.name })} danger confirm={t("Delete")}
    text={t("It won't be listed any more, and its name is free again. Its versions stay where other branches have them.")}
    onconfirm={forget} onclose={() => (forgetting = null)} />
{/if}

<style>
  .danger-text { color: var(--danger); }
  .inline { padding-top: var(--sp-4); }
  section { padding: var(--sp-12) 0; border-top: var(--border-width) solid var(--line); }
  section:first-child { border-top: none; padding-top: 0; }
  h3 { margin: 0 0 var(--sp-8); font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); font-weight: var(--fw-semibold); }
  .line { display: flex; gap: var(--sp-8); align-items: center; }
  .line input { flex: 1; }
  .branches { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
  .brow { width: 100%; display: flex; align-items: center; gap: var(--sp-8); padding: var(--sp-6) var(--sp-8); text-align: left; }
  .bdot { flex: none; width: 10px; height: 10px; border-radius: 50%; background: var(--c); }
  .bname { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .here { font-size: var(--fs-xs); color: var(--accent); }
  .brow .faint { margin-left: auto; color: var(--faint); font-size: var(--fs-sm); white-space: nowrap; }
  .fold { display: flex; align-items: center; gap: var(--sp-6); margin: var(--sp-12) 0 var(--sp-4); padding: var(--sp-4) var(--sp-8);
    font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); font-weight: var(--fw-semibold); }
  .fold .chev { transition: transform .15s; }
  .fold[aria-expanded="true"] .chev { transform: rotate(90deg); }
  .drow { display: flex; align-items: center; gap: var(--sp-8); padding: var(--sp-4) var(--sp-8); }
  .drow .faint { margin-left: auto; color: var(--faint); font-size: var(--fs-sm); white-space: nowrap; }
  .drow .bname { color: var(--muted); }
  .bdot.gone { opacity: .5; }
  button.small { padding: var(--sp-2) var(--sp-10); font-size: var(--fs-sm); }
  .hint { margin: var(--sp-6) 0 0; font-size: var(--fs-md); color: var(--muted); }
  .action .hint { margin-top: var(--sp-2); }
  dl { display: grid; grid-template-columns: 90px 1fr; gap: var(--sp-6) var(--sp-10); margin: 0; font-size: var(--fs-base); align-items: center; }
  dt { color: var(--muted); }
  dd { margin: 0; min-width: 0; }
  .folder { display: flex; gap: var(--sp-8); align-items: center; }
  .path { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; font-size: var(--fs-md); }
  .small { padding: var(--sp-2) var(--sp-10); font-size: var(--fs-md); flex: none; }
  .applied { margin: var(--sp-8) 0; padding-left: var(--sp-18); font-size: var(--fs-base); }
  .error { color: var(--warn); font-size: var(--fs-md); margin: var(--sp-6) 0; }
  .action { display: flex; align-items: center; justify-content: space-between; gap: var(--sp-16); padding: var(--sp-6) 0; }
  .action strong { font-size: var(--fs-base); font-weight: var(--fw-semibold); }
  .action button { flex: none; }
  .danger-zone h3 { color: var(--danger); }
</style>
