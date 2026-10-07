<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type ProjectInfo, type TeamProject, type TeamSummary } from "./api";
  import Modal from "./Modal.svelte";
  import RulesWindow from "./RulesWindow.svelte";
  import Tx from "./Tx.svelte";
  import { toast } from "./notify.svelte";
  import ProjectIcon from "./ProjectIcon.svelte";
  import Swatches from "./Swatches.svelte";
  import { projectColor } from "./palette";
  import { projectIconNames, projectIcons } from "./projectIcons";
  import { portal } from "./portal";
  import EmojiPicker from "./EmojiPicker.svelte";
  import { emojiName, emojiOf } from "./emoji";
  import BranchSettings from "./BranchSettings.svelte";
  import { branchLabel, branchLane } from "./branches";
  import { ago, type BranchList, type DeletedBranch } from "./api";

  // A project's settings: its name, where it is, its rules, and what can be
  // done with it (check it, unlink or delete it). The actions that
  // need a confirmation of their own are the caller's.
  let { p, team, inline = false, onclose, onrenamed, oncheck, ondelete, onunlink, onlocate }: {
    p: TeamProject;
    inline?: boolean;       // in the project's Settings tab, not a dialog
    team?: TeamSummary;     // the project's team (none: on this computer only)
    onclose: () => void;
    onrenamed: () => void;
    oncheck: () => void;
    ondelete: () => void;
    onunlink: () => void;
    onlocate: () => void;
  } = $props();

  const here = $derived(p.status === "downloaded");
  let info = $state<ProjectInfo | null>(null);
  $effect(() => {
    if (!here) return;
    api.ProjectInfo(p.root).then((i) => (info = i)).catch(() => {});
  });

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

  // Its icon and colour, for the whole team (Nightly: a team that keeps
  // looks). Each pick is saved at once.
  // svelte-ignore state_referenced_locally
  let look = $state({ icon: p.icon ?? "", color: p.color ?? "" });
  let lookBusy = $state(false);
  async function setLook(icon: string, color: string) {
    if (lookBusy) return; // one at a time: the team keeps the last pick
    const was = look;
    look = { icon, color };
    lookBusy = true;
    try {
      await api.SetProjectLook(team!.id, p.id, icon, color);
      onrenamed();
    } catch (e) {
      look = was;
      toast(errorText(e), "error");
    } finally {
      lookBusy = false;
    }
  }

  // The icon before the name opens the picker (on a team that keeps looks),
  // placed under it on the window, above when there's no room below.
  const POP_W = 392;
  let lookOpen = $state(false);
  let iconBtn = $state<HTMLButtonElement>();
  let popH = $state(0);
  let anchor = $state<DOMRect | null>(null);
  let popAt = $derived.by(() => {
    if (!anchor) return { left: 0, top: 0 };
    const left = Math.max(8, Math.min(anchor.left, innerWidth - POP_W - 8));
    const below = anchor.bottom + 6;
    return { left, top: below + popH > innerHeight - 8 && anchor.top - popH - 6 > 8 ? anchor.top - popH - 6 : below };
  });
  // Built-in icons or emoji: the picker opens on the kind the project has.
  let lookTab = $state<"icons" | "emoji">("icons");
  function toggleLook() {
    anchor = iconBtn?.getBoundingClientRect() ?? null;
    if (!lookOpen) lookTab = emojiOf(look.icon) ? "emoji" : "icons";
    lookOpen = !lookOpen;
  }
  // An icon or an emoji picked: saved, and the picker closes (a colour
  // keeps it open, for an icon next).
  async function pickIcon(icon: string) {
    if (lookBusy) return;
    closeLook(true);
    await setLook(icon, look.color);
  }
  function closeLook(refocus = false) {
    lookOpen = false;
    if (refocus) iconBtn?.focus();
  }
  // Esc closes the picker before the dialog it's in (the dialog listens on
  // the window too, after this).
  function lookKey(e: KeyboardEvent) {
    if (lookOpen && e.key === "Escape") {
      e.preventDefault();
      closeLook(true);
    }
  }
  function lookClick(e: MouseEvent) {
    const el = e.target as HTMLElement;
    if (lookOpen && !el.closest(".look-pop") && !el.closest(".look-btn")) closeLook();
  }

  // The project's branches, each opening its settings (where the team keeps
  // branch names: Nightly).
  let branchList = $state<BranchList | null>(null);
  let branchOpen = $state<string | null>(null);
  let deleted = $state<DeletedBranch[]>([]);
  const loadBranches = () => {
    api.BranchList(p.root).then((l) => (branchList = l)).catch(() => {});
    api.DeletedBranches(p.root).then((d) => (deleted = d ?? [])).catch(() => {});
  };
  let restoring = $state("");
  async function restore(d: DeletedBranch) {
    restoring = d.name;
    try {
      await api.RestoreBranch(p.root, d.name);
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
  const presetName = (preset: string) => ({ ableton: t("Ableton Live project"), unity: t("Unity project"),
    unreal: t("Unreal project"), godot: t("Godot project"), design: t("Design files"), code: t("Code"), none: t("No preset") } as Record<string, string>)[preset] ?? preset;
</script>

<svelte:window onkeydowncapture={lookKey} onclick={lookClick} onresize={() => closeLook()} />

{#if rulesOpen}
  <RulesWindow root={p.root} onclose={() => { rulesOpen = false; api.ProjectInfo(p.root).then((i) => (info = i)).catch(() => {}); }} />
{:else}
{#snippet body()}
  <section>
    <h3>{t("Name")}</h3>
    <div class="line">
      {#if team?.looks}
        <button bind:this={iconBtn} class="ghost look-btn" class:open={lookOpen} onclick={toggleLook}
          title={t("Icon and colour")} aria-label={t("Icon and colour")} aria-haspopup="dialog" aria-expanded={lookOpen}>
          <ProjectIcon p={{ id: p.id, name: p.name, status: "downloaded", icon: look.icon, color: look.color }} size={36} />
        </button>
      {:else}
        <ProjectIcon p={{ id: p.id, name: p.name, status: "downloaded", icon: p.icon, color: p.color }} size={36} />
      {/if}
      <input bind:value={name} maxlength="100" aria-label={t("Project name")}
        onkeydown={(e) => { if (e.key === "Enter" && name.trim() && name.trim() !== p.name) rename(); }} />
      <button onclick={rename} disabled={renaming || !name.trim() || name.trim() === p.name}>{renaming ? t("Renaming…") : t("Rename")}</button>
    </div>
    <p class="hint">{team ? t("Project name shared by the whole team.") : t("Project name in R3V.")} {t("Local folder keeps its name.")}</p>
  </section>

  {#if lookOpen && team?.looks}
    <div class="look-pop surface-menu" role="dialog" aria-label={t("Icon and colour")} use:portal bind:offsetHeight={popH}
      style:left="{popAt.left}px" style:top="{popAt.top}px" style:width="{POP_W}px">
      <div class="kinds" role="tablist" aria-label={t("Icon")}>
        <button role="tab" class:on={lookTab === "icons"} aria-selected={lookTab === "icons"} onclick={() => (lookTab = "icons")}>{t("Icons")}</button>
        <button role="tab" class:on={lookTab === "emoji"} aria-selected={lookTab === "emoji"} onclick={() => (lookTab = "emoji")}>{t("Emoji")}</button>
      </div>
      {#if lookTab === "icons"}
        <!-- (an emoji has its own colours: the colour is for the icons) -->
        <Swatches value={projectColor(p.id || p.name, look.color)} label={t("Colour")} disabled={lookBusy} onpick={(c) => setLook(look.icon, c)} />
        <div class="icons" role="radiogroup" aria-label={t("Icon")}>
          <button class="ic" class:on={!look.icon} role="radio" aria-checked={!look.icon} aria-disabled={lookBusy}
            title={t("The project's initial")} aria-label={t("The project's initial")} onclick={() => pickIcon("")}>
            {([...p.name.trim()][0] ?? "?").toUpperCase()}</button>
          {#each projectIconNames as name (name)}
            <button class="ic" class:on={look.icon === name} role="radio" aria-checked={look.icon === name} aria-disabled={lookBusy}
              aria-label={name} onclick={() => pickIcon(name)}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">{@html projectIcons[name]}</svg>
            </button>
          {/each}
        </div>
      {:else}
        <EmojiPicker onpick={(e) => { const n = emojiName(e); if (n) pickIcon(n); }} />
      {/if}
      <p class="hint">{t("The whole team sees them.")}</p>
    </div>
  {/if}

  {#if branchList?.names && branchList.branches.length}
    <section>
      <h3>{t("Branches")}</h3>
      <ul class="branches">
        {#each branchList.branches as b (b.name)}
          <li>
            <button class="ghost brow" onclick={() => (branchOpen = b.name)} title={t("Branch settings")}>
              <span class="bdot" style:--c="var(--lane-{branchLane(branchList.branches, b.name)})"></span>
              <span class="bname">{branchLabel(branchList.branches, b.name)}</span>
              {#if b.current}<span class="here">{t("you're on it")}</span>{/if}
              <span class="faint">{b.latest ? `${b.latest.author} · ${ago(b.latest.time)}` : ""}</span>
            </button>
          </li>
        {/each}
      </ul>
      {#if deleted.length}
        <h4>{t("Deleted branches")}</h4>
        <ul class="branches">
          {#each deleted as d (d.name)}
            <li class="drow">
              <span class="bdot gone" style:--c="var(--lane-{branchLane([d], d.name)})"></span>
              <span class="bname">{d.label || d.name}</span>
              <span class="faint">{d.by ? t("deleted by {name} {when}", { name: d.by, when: ago(d.time) }) : t("deleted {when}", { when: ago(d.time) })}</span>
              <button class="small" onclick={() => restore(d)} disabled={!!restoring}>{restoring === d.name ? t("Restoring…") : t("Restore")}</button>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {/if}

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

  {#if here && info}
    <section>
      <h3>{t("Rules")}</h3>
      <p class="hint"><Tx text={t("Which files R3V tracks, set in the project's {file}. The file is committed with the project, so everyone uses the same rules.")} code={{ file: ".r3v.yaml" }} /></p>
      <ul class="applied">
        {#each info.rules.applied as a}
          <li><strong>{presetName(a.preset)}</strong>
            <span class="faint">{a.folder ? t("in {folder}", { folder: `${a.folder}/` }) : t("the project folder")}{a.detected ? ` · ${t("detected")}` : ""}</span></li>
        {:else}
          <li class="faint">{t("No preset: every file is tracked.")}</li>
        {/each}
      </ul>
      {#if info.rules.error}<p class="error">⚠ {info.rules.error}</p>{/if}
      <div class="line">
        <button class="primary" onclick={() => (rulesOpen = true)}>{t("Rules…")}</button>
        <button onclick={openRules}>{t("Edit as text")}</button>
        <button class="ghost" onclick={() => api.OpenURL("https://github.com/nonlabhq/R3V/blob/main/docs/profiles.md")}>{t("Guide ↗")}</button>
      </div>
    </section>

    <section>
      <h3>{t("Health")}</h3>
      <div class="action">
        <div><strong>{t("Check project…")}</strong><p class="hint">{t("Reads its whole history again, looking for damage.")}</p></div>
        <button onclick={oncheck}>{t("Check…")}</button>
      </div>
    </section>
  {/if}

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
    {#if team}
      <div class="action">
        <div><strong>{t("Delete from {team}…", { team: team.name })}</strong>
          <p class="hint">{t("Removes it and all its versions from the team, for everyone.")}</p></div>
        <button class="danger" onclick={ondelete}>{t("Delete…")}</button>
      </div>
    {/if}
  </section>

{/snippet}
{#if inline}
  <div class="inline">{@render body()}</div>
{:else}
<Modal title={t("Project settings")} {onclose} width={600}>
  {@render body()}
  {#snippet footer()}
    <button onclick={onclose}>{t("Close")}</button>
  {/snippet}
</Modal>
{/if}
{/if}

{#if branchOpen !== null && branchList}
  <BranchSettings root={p.root} branch={branchOpen} branches={branchList.branches}
    onchanged={() => { loadBranches(); onrenamed(); }} onclose={() => (branchOpen = null)} />
{/if}

<style>
  .inline { padding-top: var(--sp-4); }
  section { padding: var(--sp-12) 0; border-top: var(--border-width) solid var(--line); }
  section:first-child { border-top: none; padding-top: 0; }
  h3 { margin: 0 0 var(--sp-8); font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); font-weight: var(--fw-semibold); }
  .line { display: flex; gap: var(--sp-8); align-items: center; }
  .line input { flex: 1; }
  .look-btn { flex: none; padding: 0; line-height: 0; border-radius: var(--radius); }
  .look-btn:hover:not(:disabled), .look-btn.open { outline: 2px solid var(--line-strong); outline-offset: 2px; background: transparent; }
  .look-pop { position: fixed; z-index: var(--z-menu); display: flex; flex-direction: column; gap: var(--sp-10);
    padding: var(--sp-12); border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop); }
  .look-pop .hint { margin: 0; }
  .branches { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
  .brow { width: 100%; display: flex; align-items: center; gap: var(--sp-8); padding: var(--sp-6) var(--sp-8); text-align: left; }
  .bdot { flex: none; width: 10px; height: 10px; border-radius: 50%; background: var(--c); }
  .bname { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .here { font-size: var(--fs-xs); color: var(--accent); }
  .brow .faint { margin-left: auto; color: var(--faint); font-size: var(--fs-sm); white-space: nowrap; }
  h4 { margin: var(--sp-12) 0 var(--sp-4); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); font-weight: var(--fw-semibold); }
  .drow { display: flex; align-items: center; gap: var(--sp-8); padding: var(--sp-4) var(--sp-8); }
  .drow .faint { margin-left: auto; color: var(--faint); font-size: var(--fs-sm); white-space: nowrap; }
  .drow .bname { color: var(--muted); }
  .bdot.gone { opacity: .5; }
  button.small { padding: var(--sp-2) var(--sp-10); font-size: var(--fs-sm); }
  .kinds { display: flex; gap: var(--sp-2); padding: var(--sp-2); border-radius: var(--radius); background: var(--bg-sunken); align-self: flex-start; }
  .kinds button { padding: var(--sp-2) var(--sp-10); font-size: var(--fs-sm); border-color: transparent; background: transparent; color: var(--muted); }
  .kinds button.on { background: var(--panel-2); color: var(--text); }
  .icons { display: grid; grid-template-columns: repeat(auto-fill, 32px); gap: var(--sp-4); }
  .ic { width: 32px; height: 32px; padding: 0; display: inline-flex; align-items: center; justify-content: center;
    font-weight: var(--fw-bold); font-size: var(--fs-md); color: var(--muted); }
  .ic svg { width: 18px; height: 18px; }
  .ic.on { color: var(--accent); border-color: var(--accent); background: var(--accent-soft); }
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
