<script lang="ts">
  import TeamMark from "$shared/TeamMark.svelte";
  import { onMount } from "svelte";
  import { Events, Window } from "@wailsio/runtime";
  import { api, errorText, formatBytes, progressShort, titleOf, unknownKind, type Overview, type Progress, type ProjectInfo, type TeamProject, type TeamSummary, type Profile } from "./lib/api";
  import type { DownloadSize, UpdateInfo, UpdateState } from "../bindings/github.com/nonlabhq/r3v/desktop/models";
  import ProgressBar from "./lib/ProgressBar.svelte";
  import { toast } from "./lib/notify.svelte";
  import ProjectView from "./lib/ProjectView.svelte";
  import TeamHome from "./lib/TeamHome.svelte";
  import QuickLaunch, { type Pick as LaunchPick } from "./lib/QuickLaunch.svelte";
  import { linkPick } from "./lib/links.svelte";
  import { besideMain, moveKey, readOrder, sortByOrder } from "./lib/order";
  import { addTab, closeTab, homeRef, isHome, isNew as isNewTab, loadTabs, moveTab, oldTabs, pruneTabs, refreshSnaps, renameTab, replaceTab,
    snapOf, tabId, teamsOf, toSave, type TabRef, type TabSnap } from "./lib/tabs";
  import ProjectIcon from "$shared/ProjectIcon.svelte";
  import { tick, untrack } from "svelte";
  import { flip } from "svelte/animate";
  import { spinner } from "./lib/spin.svelte";
  import KeptSamples from "./lib/KeptSamples.svelte";
  import Onboarding from "./lib/Onboarding.svelte";
  import TeamMenu from "./lib/TeamMenu.svelte";
  import TeamSettings from "./lib/TeamSettings.svelte";
  import VerifyDialog from "./lib/VerifyDialog.svelte";
  import Modal from "$shared/Modal.svelte";
  import Toasts from "./lib/Toasts.svelte";
  import StyleLab from "./lib/StyleLab.svelte";
  import ProjectSettings from "./lib/ProjectSettings.svelte";
  import AppSettings from "./lib/AppSettings.svelte";
  import { t, tn } from "./lib/i18n.svelte";
  import PreuploadIcon from "./lib/PreuploadIcon.svelte";
  import { preuploads, queue, watchPreuploads } from "./lib/preupload.svelte";
  import UploadQueue from "./lib/UploadQueue.svelte";
  import KeysHelp from "./lib/KeysHelp.svelte";
  import Tooltip from "./lib/Tooltip.svelte";
  import Avatar from "$shared/Avatar.svelte";
  import UserSettings from "./lib/UserSettings.svelte";
  import KindDialog from "./lib/KindDialog.svelte";
  import ConfirmDialog from "$shared/ConfirmDialog.svelte";


  let overview = $state<Overview | null>(null);
  let onboarding = $state(false);
  // Selected sidebar entry: a project folder, or a team project not here yet.
  let selected = $state<{ root?: string; id?: string }>({});
  let refreshKey = $state(0);
  let busy = $state("");
  let autostart = $state(false);
  let rowMenu = $state(""); // key of the project whose menu (right-click) is open
  let menuAt = $state({ left: 0, top: 0 }); // where it was asked for
  let confirmDelete = $state<TeamProject | null>(null);
  // A project about to leave R3V: teammates' samples are offered into it first.
  let leaving = $state<{ roots: string[]; go: () => void } | null>(null);
  let checking = $state<TeamProject | null>(null); // Check project…
  let deleteWord = $state("");
  const rowKey = (p: TeamProject) => p.root || p.id;
  let settingsFor = $state<TeamProject | null>(null); // Project settings
  // A team's settings: from its ⚙ in the sidebar, or a project's header.
  let teamSettingsFor = $state<TeamSummary | null>(null);
  // Closes the settings and gives their project: read it first (what the
  // dialog shows goes with settingsFor).
  function closeSettings(): TeamProject {
    const p = settingsFor!;
    settingsFor = null;
    return p;
  }
  // The menu's "Open in" (read when the menu opens).
  let menuInfo = $state<ProjectInfo | null>(null);
  let openSub = $state(false);
  // The list of sets beside the menu: placed on the window (the sidebar's
  // list scrolls, and would cut it off), kept open while the pointer
  // crosses over to it.
  let subAt = $state({ left: 0, top: 0 });
  let subTimer: ReturnType<typeof setTimeout> | undefined;
  function showSub(e: MouseEvent) {
    clearTimeout(subTimer);
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    subAt = { left: r.right - 2, top: r.top - 6 };
    openSub = true;
  }
  function hideSub() {
    clearTimeout(subTimer);
    subTimer = setTimeout(() => (openSub = false), 180);
  }
  function openMenu(p: TeamProject, e: MouseEvent) {
    e.preventDefault();
    const key = rowKey(p);
    rowMenu = key;
    menuAt = { left: Math.min(e.clientX, window.innerWidth - 240), top: Math.min(e.clientY, window.innerHeight - 160) };
    menuInfo = null;
    openSub = false;
    if (rowMenu && p.root && p.status === "downloaded") {
      api.ProjectInfo(p.root).then((i) => { if (rowMenu === key) menuInfo = i; }).catch(() => {});
    }
  }
  const toolName = (tool: string) => (tool === "Ableton Live" ? "Live" : tool ? t(tool) : t("its program"));
  function openIn(p: TeamProject, rel: string) {
    rowMenu = "";
    api.OpenInTool(p.root, rel).catch((e) => toast(errorText(e), "error"));
  }
  const openLabel = (p: TeamProject, rel: string) => (rel === "." ? p.name : rel);

  const SELECTED_KEY = "r3v.selected";
  const DOWNLOAD_DIR_KEY = "r3v.downloadDir";
  const readJSON = (k: string): unknown => { try { return JSON.parse(localStorage.getItem(k) ?? "null"); } catch { return null; } };
  const remember = (k: string, v: string) => { try { localStorage.setItem(k, v); } catch { /* not persisted */ } };
  const recall = (k: string) => { try { return localStorage.getItem(k) ?? ""; } catch { return ""; } };

  let reloading = $state(false);
  // (it turns while the team is asked, too: what the sidebar shows of it then)
  const turning = spinner(() => reloading || (!!overview?.currentTeam && !overview.teamChecked));
  let lastReload = 0;
  async function reload() {
    reloading = true;
    lastReload = Date.now();
    try {
      // At first from this computer alone, at once: asking the team can
      // take a while (offline, storage down), and it fills in after.
      if (!overview) overview = await api.LocalOverview();
      overview = await api.Overview();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      reloading = false;
    }
  }

  // New projects on the team show up without a manual refresh: every minute,
  // and when the window comes back to the front.
  function reloadIfStale() {
    if (!onboarding && !reloading && Date.now() - lastReload > 5000) reload();
  }
  $effect(() => {
    const t = setInterval(reloadIfStale, 60000);
    return () => clearInterval(t);
  });

  // Save/upload/download progress per project folder, for the sidebar.
  let activity = $state<Record<string, Progress>>({});
  let lastProgress = $state<Progress | null>(null);
  const activityTimers: Record<string, ReturnType<typeof setTimeout>> = {};
  function onProgress(p: Progress) {
    clearTimeout(activityTimers[p.root]);
    if (p.stage === "done") {
      delete activity[p.root];
      return;
    }
    activity[p.root] = p;
    lastProgress = p;
    // In case the final event is missed.
    activityTimers[p.root] = setTimeout(() => delete activity[p.root], 10 * 60000);
  }

  // A project just added to a team: its view commits and uploads the first
  // version.
  let firstShare = $state("");
  let justDownloaded = $state(""); // show its check when it opens
  let appVersion = $state("");
  function copyVersion() {
    navigator.clipboard?.writeText(appVersion).then(() => toast(t("Copied {version}", { version: appVersion }), "ok"),
      () => toast(appVersion, "info"));
  }
  let edition = $state(""); // a build with extensions, e.g. "Pro"
  let nightly = $state(false); // the Style lab is there to try looks

  // A newer release: offered until the user hides that version (unless it's
  // required: then the app can't be used before updating). A signed one is
  // downloaded in the background and installed by R3V itself.
  let update = $state<UpdateInfo | null>(null);
  let updState = $state<UpdateState>({ stage: "", done: 0, total: 0, error: "", auto: true });
  let installWhenReady = $state(false); // "Update now" before the download finished
  const DISMISSED_KEY = "r3v.dismissedUpdate";
  async function checkUpdate() {
    try {
      const u = await api.CheckUpdate();
      update = u && (u.required || u.version !== recall(DISMISSED_KEY)) ? u : null;
      updState = await api.UpdateStatus();
    } catch {
      /* offline: try again later */
    }
  }
  let anyBusy = $derived(Object.keys(activity).length > 0);
  // While it downloads, ask how it goes too (an event can come before the
  // first answer and be overwritten by it).
  $effect(() => {
    if (updState.stage !== "downloading") return;
    const t = setInterval(() => api.UpdateStatus().then(onUpdateState).catch(() => {}), 1000);
    return () => clearInterval(t);
  });
  async function updateNow() {
    if (updState.stage === "ready") {
      try {
        await api.InstallUpdate(); // R3V closes, updates and opens again
      } catch (e) {
        toast(errorText(e), "error", 8000);
      }
      return;
    }
    installWhenReady = true;
    api.DownloadUpdate().catch(() => {});
  }
  function onUpdateState(st: UpdateState) {
    updState = st;
    if (st.stage === "ready" && installWhenReady && !anyBusy) {
      installWhenReady = false;
      updateNow();
    }
  }
  const pct = (st: UpdateState) => (st.total > 0 ? Math.round((st.done / st.total) * 100) : 0);
  // The settings' "Check for updates": now, and say when there's none.
  let appSettings = $state(false);
  // The user's colour and picture (Nightly), and their settings.
  let profile = $state<Profile | null>(null);
  let userSettings = $state(false);
  function loadProfile() { api.Profile().then((p) => (profile = p)).catch(() => {}); }
  let checkingNow = $state(false);
  async function checkUpdateNow() {
    checkingNow = true;
    try {
      const u = await api.CheckUpdateNow();
      update = u;
      updState = await api.UpdateStatus();
      if (!u) toast(t("You have the newest version"), "ok");
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      checkingNow = false;
    }
  }
  async function setAutoUpdate(on: boolean) {
    try {
      await api.SetAutoUpdate(on);
      updState = { ...updState, auto: on };
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
  $effect(() => {
    const t = setInterval(checkUpdate, 6 * 3600 * 1000);
    return () => clearInterval(t);
  });
  function openLink(url: string) {
    api.OpenURL(url).catch(() => window.open(url, "_blank"));
  }

  let current = $derived(overview?.teams.find((t) => t.id === overview?.currentTeam));
  // Every project is a team's (0.9: no projects kept on this computer only),
  // in the order dragged in the sidebar (each team its own, this computer
  // only).
  const orderKey = (team: string) => `r3v.order:${team}`;
  let order = $state<string[]>([]);
  $effect.pre(() => { order = readOrder(readJSON(orderKey(overview?.currentTeam ?? ""))); });
  // (by the project's id: it stays when the project is downloaded)
  // (an agent's workspace, by its folder: it has its project's id; it stays
  // beside its project, not dragged)
  const orderOf = (p: TeamProject) => (p.workspace ? p.root : p.id || p.root);
  let entries = $derived(besideMain(sortByOrder(overview?.projects ?? [], orderOf, order)));
  // A project's workspaces, under it in the sidebar, folded away with its
  // toggle (remembered, by the project's folder).
  const sameRoot = (a = "", b = "") => !!a && !!b && a.replace(/[\\/]+$/, "").toLowerCase() === b.replace(/[\\/]+$/, "").toLowerCase();
  let wsFolded = $state<Record<string, boolean>>((() => { try { return JSON.parse(localStorage.getItem("r3v.wsFolded") ?? "{}") ?? {}; } catch { return {}; } })());
  function foldWorkspaces(p: TeamProject) {
    wsFolded[p.root] = !wsFolded[p.root];
    remember("r3v.wsFolded", JSON.stringify(wsFolded));
  }
  const workspacesOf = (p: TeamProject) => (p.workspace || !p.root ? [] : entries.filter((w) => w.workspace && sameRoot(w.mainRoot, p.root)));
  const wsHidden = (p: TeamProject) => !!p.workspace && !!wsFolded[entries.find((m) => !m.workspace && sameRoot(m.root, p.mainRoot))?.root ?? ""];
  // (the one shown stays, folded or not)
  let rows = $derived(entries.filter((p) => !wsHidden(p) || p === selectedEntry));
  let selectedEntry = $derived(
    entries.find((p) => (selected.root ? p.root === selected.root : !!selected.id && p.id === selected.id && !p.root)));

  // The team's home (a click on the team): shown instead of a project, in a
  // tab of its own like a project (none once every tab is closed); its
  // projects, or its settings.
  let home = $state(false);
  let homeSection = $state<"projects" | "settings">("projects");
  function showHome(section: "projects" | "settings" = "projects") {
    home = true;
    homeSection = section;
  }
  // (in the tab shown, as a left click in the sidebar does)
  function goHome(section: "projects" | "settings" = "projects") {
    if (teamId) openHere(homeRef(teamId));
    showHome(section);
  }
  // A team picked in the team menu: its home in the tab shown, as a left
  // click in the sidebar does (or its home's tab, open already); behind (a
  // middle click or Ctrl+click), its home in a tab behind.
  async function switchTeam(id: string, behind: boolean) {
    if (behind) return openTab(homeRef(id));
    const from = tabItems.find(activeTab)?.id ?? "";
    if (id !== teamId) {
      switching = true;
      try {
        await showTeam(id);
      } catch (e) {
        toast(errorText(e), "error");
        return;
      } finally {
        switching = false;
      }
    }
    tabsClosed = false;
    const got = replaceTab(tabs, from, homeRef(id));
    if (got !== tabs) { tabs = got; saveTabs(); }
    showHome();
  }
  // A team's settings: the shown team's in its home, another's in a dialog.
  function openTeamSettings(team: TeamSummary) {
    if (team.id === overview?.currentTeam) goHome("settings");
    else teamSettingsFor = team;
  }

  // Projects opened last, of any team, newest first (the quick launcher
  // offers them).
  const RECENT_KEY = "r3v.recent";
  let recent = $state<{ team: string; key: string }[]>((() => {
    const v = readJSON(RECENT_KEY);
    return Array.isArray(v) ? v.filter((x) => x && typeof x.team === "string" && typeof x.key === "string") : [];
  })());

  // Shows a project of the shown team, in a tab of its own (open already, or
  // opened at the end).
  function select(p: TeamProject) {
    openTab(projRef(p));
    show(p);
  }
  // A click on a project in the sidebar: it shows in the tab shown; a middle
  // click or Ctrl+click opens it in a tab behind, as a browser does.
  function pick(p: TeamProject, e: MouseEvent) {
    if (e.ctrlKey || e.metaKey) return openTab(projRef(p));
    openHere(projRef(p));
    show(p);
  }
  // (a middle click: no autoscroll, and the click's own step)
  const noAutoscroll = (e: MouseEvent) => { if (e.button === 1) e.preventDefault(); };
  const middle = (go: () => void) => (e: MouseEvent) => { if (e.button === 1) { e.preventDefault(); go(); } };
  function show(p: TeamProject) {
    selected = p.root ? { root: p.root } : { id: p.id };
    if (p.root) remember(SELECTED_KEY, p.root);
    home = false;
    if (teamId) {
      recent = [{ team: teamId, key: rowKey(p) }, ...recent.filter((x) => !(x.team === teamId && x.key === rowKey(p)))].slice(0, 12);
      remember(RECENT_KEY, JSON.stringify(recent));
    }
  }

  // Projects and teams' homes open as tabs at the top: one list for every
  // team (remembered; drag one to move it). A click in the sidebar shows one
  // of its team's in the tab shown (or goes to its tab when it is open
  // already), a middle click opens it in a tab behind; a tab of another team
  // switches the sidebar to that team first ("the sidebar follows the
  // tab"). Ctrl+T or + opens the quick launcher (a project or a team's home,
  // of any team), in a tab of its own.
  const TABS_KEY = "r3v.tabs";
  const savedTabs = readJSON(TABS_KEY);
  let tabs = $state<TabRef[]>(loadTabs(savedTabs, []));
  // Before one list for all, each team had its own: the first team shown
  // starts the list (the others' old lists are left as they were).
  let migrateTabs = savedTabs === null;
  let launching = $state(false); // the quick launcher open
  let tabsClosed = $state(false); // every tab closed: nothing is picked for you
  let switching = $state(false); // going to another team's tab: nothing is picked for you
  let teamId = $derived(overview?.currentTeam ?? "");
  $effect.pre(() => {
    if (!teamId) return;
    if (migrateTabs) {
      migrateTabs = false;
      tabs = loadTabs([...oldTabs(teamId, readJSON(`r3v.tabs:${teamId}`)), ...untrack(() => tabs)], untrack(() => tabs));
      saveTabs();
    }
    tabsClosed = false;
  });
  // The shown team's tabs keep a picture of their projects, for when another
  // team is shown.
  $effect(() => {
    const got = refreshSnaps(untrack(() => tabs), teamId, new Map(entries.map((p) => [rowKey(p), snapOf(p)])));
    if (got !== untrack(() => tabs)) { tabs = got; saveTabs(); }
  });
  // Tabs of teams no longer on this computer go, and of projects the shown
  // team no longer lists (once it has answered, and was reachable: its list
  // is whole).
  $effect(() => {
    if (!overview) return;
    const whole = overview.teamChecked && !overview.teamError;
    const pruned = pruneTabs(untrack(() => tabs), new Set(overview.teams.map((t) => t.id)), teamId, whole ? new Set(entries.map(rowKey)) : null);
    if (pruned.length !== untrack(() => tabs).length) { tabs = pruned; saveTabs(); }
  });
  // A tab drawn: the shown team's are drawn from its list (only once it
  // lists them), another team's from the picture kept.
  // A team's home: drawn from the team (while it is on this computer).
  type Tab = { id: string; key: string; team: string; p?: TeamProject; snap?: TabSnap; home?: boolean };
  let tabItems = $derived(tabs.map((x): Tab | null => {
    const id = tabId(x);
    if (isNewTab(x.key)) return null; // (new tabs were before the quick launcher)
    if (isHome(x.key)) return overview?.teams.some((t) => t.id === x.team) ? { id, key: x.key, team: x.team, home: true } : null;
    if (x.team !== teamId) return x.p ? { id, key: x.key, team: x.team, snap: x.p } : null;
    const p = entries.find((e) => rowKey(e) === x.key);
    return p ? { id, key: x.key, team: x.team, p } : null;
  }).filter((x): x is Tab => !!x));
  // Which team a tab is on shows once tabs of more than one are open.
  let manyTeams = $derived(teamsOf(tabItems) > 1);
  const teamOf = (id: string) => overview?.teams.find((t) => t.id === id);
  // "Team · Project", and its folder.
  const tabTitle = (team: string, p: { name: string; root: string }) =>
    [team ? `${team} · ${p.name}` : p.name, p.root].filter(Boolean).join("\n");
  // (by key: reading the overview again makes new project objects)
  const activeTab = (x: Tab) => (x.home ? home && x.team === teamId : !!x.p && !home && !!selectedEntry && rowKey(selectedEntry) === x.key);
  function saveTabs() { remember(TABS_KEY, JSON.stringify(toSave(tabs))); }
  const isOpen = (p: TeamProject) => tabs.some((x) => x.team === teamId && x.key === rowKey(p));
  const projRef = (p: TeamProject): TabRef => ({ team: teamId, key: rowKey(p), p: snapOf(p) });
  // A tab of its own for ref, at the end, not shown (shown: the caller's).
  function openTab(ref: TabRef) {
    tabsClosed = false;
    if (!ref.team) return;
    const got = addTab(tabs, ref);
    if (got !== tabs) { tabs = got; saveTabs(); }
  }
  // ref in the tab shown instead of what it shows (none shown, or ref open
  // already: as openTab); the caller shows it.
  function openHere(ref: TabRef) {
    tabsClosed = false;
    if (!ref.team) return;
    const got = replaceTab(tabs, tabItems.find(activeTab)?.id ?? "", ref);
    if (got !== tabs) { tabs = got; saveTabs(); }
  }
  // A project's key changed (downloaded, found elsewhere, unlinked): its tab
  // stays where it was.
  function renamed(from: string, to: string) {
    tabs = renameTab(tabs, teamId, from, to);
    saveTabs();
  }
  // From the quick launcher: a team's home, or a project (its team shown
  // first).
  // r3v:// links (R3V Cloud's web app; Nightly): a team, a project, a
  // version, opened as the quick launcher opens them; what this computer
  // hasn't is said.
  async function openLinks() {
    for (let l = await api.TakeLink().catch(() => null); l; l = await api.TakeLink().catch(() => null)) {
      if (l.missing === "team") { toast(t("This computer hasn't joined that team: join it in R3V first."), "info", 8000); continue; }
      if (l.missing === "project") { toast(t("No team on this computer has that project."), "info", 8000); continue; }
      if (l.version) { linkPick.project = l.project; linkPick.version = l.version; }
      await launch({ team: l.team, project: l.project ? ({ id: l.project, root: "" } as TeamProject) : undefined });
    }
  }

  async function launch(pick: LaunchPick) {
    launching = false;
    if (pick.team !== teamId) {
      switching = true;
      try {
        await showTeam(pick.team);
      } catch (e) {
        toast(errorText(e), "error");
        return;
      } finally {
        switching = false;
      }
    }
    const want = pick.project;
    if (!want) { openTab(homeRef(teamId)); return showHome(); }
    const p = entries.find((e) => rowKey(e) === rowKey(want)) ?? entries.find((e) => !!want.id && e.id === want.id);
    if (p) select(p);
  }
  // Another team's tab: that team is shown (as the team menu does), then its
  // project, once the team lists it.
  async function goTab(x: Tab) {
    if (x.home && x.team === teamId) return showHome();
    if (x.p) return select(x.p);
    switching = true;
    try {
      await showTeam(x.team);
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      switching = false;
    }
    if (teamId !== x.team) return;
    if (x.home) return showHome();
    const p = entries.find((e) => rowKey(e) === x.key);
    if (p) select(p);
  }
  // Another team shown at once: its projects as this computer knows them
  // (LocalOverview), the team asked after, in the background (reload).
  async function showTeam(id: string) {
    await api.SelectTeam(id);
    overview = await api.LocalOverview();
    reload();
  }
  // Tabs closed, newest last: Ctrl+Shift+T opens the last one again (on its
  // team, while that team is still here).
  let closedTabs: TabRef[] = [];
  function reopenTab() {
    const x = closedTabs.pop();
    if (!x) return;
    if (x.team === teamId && !isHome(x.key)) {
      const p = entries.find((e) => rowKey(e) === x.key);
      if (p) select(p);
    } else if (teamOf(x.team) && (x.p || isHome(x.key))) {
      openTab(x);
      goTab({ id: tabId(x), key: x.key, team: x.team, snap: x.p, home: isHome(x.key) });
    }
  }
  function closeTabOf(x: Tab) {
    const ref = tabs.find((y) => tabId(y) === x.id);
    if (ref && x.team) closedTabs = [...closedTabs.filter((y) => tabId(y) !== x.id), ref].slice(-20);
    const wasActive = activeTab(x);
    const { next } = closeTab(tabItems.map((y) => y.id), x.id);
    const n = tabItems.find((y) => y.id === next);
    tabs = tabs.filter((y) => tabId(y) !== x.id);
    saveTabs();
    if (!wasActive) return;
    if (n?.p) select(n.p);
    else if (n?.team) { selected = {}; goTab(n); }
    else { selected = {}; tabsClosed = true; showHome(); }
  }
  // Dragging a tab moves it among the others (a drag isn't a click). It
  // ends when the button is let go, wherever, or the drag is cancelled.
  let tabDrag: { key: string; x: number; moved: boolean } | null = null;
  let tabDragged = false;
  let tabsEl = $state<HTMLElement>();
  function tabDown(e: PointerEvent, key: string) {
    if (e.button !== 0 || (e.target as HTMLElement).closest(".tab-x")) return;
    tabDrag = { key, x: e.clientX, moved: false };
    const move = (m: PointerEvent) => {
      if (!tabDrag) return;
      if (!(m.buttons & 1)) return up();
      if (!tabDrag.moved && Math.abs(m.clientX - tabDrag.x) < 5) return;
      tabDrag.moved = true;
      const els = [...(tabsEl?.querySelectorAll<HTMLElement>(".tab") ?? [])];
      let to = els.findIndex((el) => { const r = el.getBoundingClientRect(); return m.clientX < r.left + r.width / 2; });
      if (to < 0) to = els.length;
      const moved = moveTab(tabs, tabItems.map((x) => x.id), tabDrag.key, to);
      if (moved !== tabs) tabs = moved;
    };
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", up);
      if (tabDrag?.moved) { tabDragged = true; saveTabs(); setTimeout(() => (tabDragged = false)); }
      tabDrag = null;
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", up);
  }
  // Dragging a project in the sidebar moves it (a drag isn't a click); the
  // order is kept for its team on this computer. It follows the pointer and
  // takes its place among the others as it goes, which slide out of its way
  // (animate:flip); let go, it settles into that place.
  let projDrag: { key: string; y: number; moved: boolean } | null = null;
  let projDragged = false;
  let dragging = $state(""); // the project being dragged (drawn lifted)
  let dragDy = $state(0); // how far it is from its place, following the pointer
  const reducedMotion = () => typeof matchMedia === "function" && matchMedia("(prefers-reduced-motion: reduce)").matches;
  // (the dragged one doesn't slide: it is where the pointer is)
  const slide = (key: string) => ({ duration: dragging === key || reducedMotion() ? 0 : 160 });
  function projDown(e: PointerEvent, key: string) {
    if (e.button !== 0 || (e.target as HTMLElement).closest(".fold-ws, .row-menu")) return;
    const row = e.currentTarget as HTMLElement;
    const list = row.closest<HTMLElement>("[data-projects]");
    if (!list) return;
    // Where a row's place is on screen: offsetTop leaves out the slides and
    // the follow (transforms), so places don't swing while rows move.
    const place = (el: HTMLElement) => list.getBoundingClientRect().top + el.offsetTop - list.offsetTop - list.scrollTop;
    const grab = e.clientY - place(row);
    projDrag = { key, y: e.clientY, moved: false };
    const move = (m: PointerEvent) => {
      if (!projDrag) return;
      if (!(m.buttons & 1)) return up();
      if (!projDrag.moved && Math.abs(m.clientY - projDrag.y) < 5) return;
      projDrag.moved = true;
      dragging = key;
      // Before the first other row whose middle is below the dragged one's.
      const mid = m.clientY - grab + row.offsetHeight / 2;
      const before = [...list.querySelectorAll<HTMLElement>("[data-key]")]
        .find((el) => el !== row && mid < place(el) + el.offsetHeight / 2)?.dataset.key;
      const keys = entries.map(orderOf);
      const at = before ? keys.indexOf(before) : -1;
      const moved = moveKey(keys, key, at < 0 ? keys.length : at);
      if (moved !== keys) order = moved;
      tick().then(() => { if (dragging === key) dragDy = m.clientY - grab - place(row); });
    };
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", up);
      if (projDrag?.moved) {
        projDragged = true;
        remember(orderKey(teamId), JSON.stringify(order));
        setTimeout(() => (projDragged = false));
      }
      projDrag = null;
      dragging = "";
      dragDy = 0;
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", up);
  }
  // The sidebar folds to the logo and a button to open it again (remembered).
  const SIDEBAR_KEY = "r3v.sidebar";
  let folded = $state(recall(SIDEBAR_KEY) === "folded");
  function fold(on: boolean) { folded = on; remember(SIDEBAR_KEY, on ? "folded" : ""); }
  // Its edge is dragged to size it; past its narrowest by a stretch it
  // folds, and dragged out from folded it opens again (remembered).
  const SIDE_W_KEY = "r3v.sidebarWidth", SIDE_MIN = 200, SIDE_MAX = 420, SIDE_FOLD = 60;
  let sideW = $state(Math.min(SIDE_MAX, Math.max(SIDE_MIN, Number(recall(SIDE_W_KEY)) || 256)));
  let sizing = $state(false);
  function sideDrag(e: PointerEvent) {
    if (e.button !== 0) return;
    e.preventDefault();
    sizing = true;
    const move = (m: PointerEvent) => {
      if (!(m.buttons & 1)) return up();
      if (m.clientX < SIDE_MIN - SIDE_FOLD) { if (!folded) fold(true); return; }
      if (folded) fold(false);
      sideW = Math.round(Math.min(SIDE_MAX, Math.max(SIDE_MIN, m.clientX)));
    };
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", up);
      sizing = false;
      remember(SIDE_W_KEY, String(sideW));
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", up);
  }
  // The window's own controls (no Windows title bar): in the browser
  // (server mode) they do nothing.
  const win = (f: () => Promise<unknown>) => f().catch(() => {});

  // Keep a valid selection when the team or the list changes.
  $effect(() => {
    if (!overview || selectedEntry || tabsClosed || home || switching) return;
    const last = recall(SELECTED_KEY);
    // (the first of its tabs: a project, or its home)
    const first = untrack(() => tabItems).find((x) => x.p || (x.home && x.team === teamId));
    if (first?.home) return showHome();
    const pick = first?.p ?? entries.find((p) => p.root && p.root === last) ??
      entries.find((p) => p.status === "downloaded") ?? entries[0];
    if (pick) select(pick);
  });

  // Where downloads go, and the size of the selected project.
  let downloadDir = $state(recall(DOWNLOAD_DIR_KEY));
  let dlSize = $state<{ id: string; size: DownloadSize | null; error: string } | null>(null);
  // The team's projects not downloaded here are listed as it last listed
  // them while it's being asked or can't be reached: not to be downloaded.
  let teamLocked = $derived(!overview?.teamChecked || !!overview?.teamError);
  $effect(() => {
    const p = selectedEntry;
    const team = overview?.currentTeam;
    if (!p || p.status !== "remote" || !team || teamLocked) return;
    const id = p.id;
    dlSize = { id, size: null, error: "" };
    api.ProjectDownloadSize(team, id)
      .then((size) => { if (dlSize?.id === id) dlSize = { id, size, error: "" }; })
      .catch((e) => { if (dlSize?.id === id) dlSize = { id, size: null, error: errorText(e) }; });
  });

  async function download(p: TeamProject) {
    let parent = downloadDir;
    if (!parent) {
      parent = await api.ChooseFolder(t("Where should downloaded projects go?"));
      if (!parent) return;
      remember(DOWNLOAD_DIR_KEY, parent);
      downloadDir = parent;
    }
    busy = p.id;
    lastProgress = null;
    try {
      const got = await api.DownloadProject(overview!.currentTeam, p.id, parent);
      toast(t("Downloaded “{name}” to {folder}", { name: got.name, folder: got.root }), "ok", 7000);
      justDownloaded = got.root;
      renamed(rowKey(p), rowKey(got));
      await reload();
      select(got);
    } catch (e) {
      toast(errorText(e), "error", 9000);
    } finally {
      busy = "";
    }
  }

  async function changeDownloadDir() {
    const d = await api.ChooseFolder(t("Where should downloaded projects go?"));
    if (d) {
      remember(DOWNLOAD_DIR_KEY, d);
      downloadDir = d;
    }
  }

  async function locate(p: TeamProject) {
    const folder = await api.ChooseFolder(t("Where is “{name}” now?", { name: p.name }));
    if (!folder) return;
    try {
      const got = await api.LocateProject(overview!.currentTeam, p.id, folder);
      renamed(rowKey(p), rowKey(got));
      await reload();
      select(got);
    } catch (e) {
      toast(errorText(e), "error");
    }
  }

  async function forget(p: TeamProject) {
    await api.ForgetProject(p.root);
    toast(t("Unlinked “{name}”: the folder and its versions are untouched", { name: p.name }), "info");
    selected = {};
    if (p.id) renamed(rowKey(p), p.id); // on the team still: its tab shows it there
    await reload();
  }

  // A workspace just shown in the sidebar (from its project's Agents tab): go to it.
  async function openWorkspace(p: TeamProject) {
    await reload();
    select(entries.find((e) => e.root === p.root) ?? p);
  }

  function leaveThenDelete(p: TeamProject) {
    confirmDelete = null;
    if (p.root) leaving = { roots: [p.root], go: () => deleteFromTeam(p) };
    else deleteFromTeam(p);
  }

  async function deleteFromTeam(p: TeamProject) {
    confirmDelete = null;
    busy = "delete";
    try {
      await api.DeleteProjectFromTeam(overview!.currentTeam, p.id);
      toast(t(p.root ? "Deleted “{name}” from {team}. Your copy stays in its folder." : "Deleted “{name}” from {team}",
        { name: p.name, team: current?.name ?? "" }), "info", 8000);
      if (!p.root) selected = {};
      await reload();
    } catch (e) {
      toast(errorText(e), "error", 9000);
    } finally {
      busy = "";
    }
  }

  async function addToTeam() {
    // No confirmation here: the project asks before sharing anything.
    const folder = await api.ChooseFolder(t("Choose a project folder to add to {team}", { team: current?.name ?? t("the team") }));
    if (folder) share(folder);
  }

  // A project just added (Nightly): the guide for AI agents offered when it
  // has none (AGENTS.md: committed with its first version).
  // theirs: the project has an AGENTS.md (or CLAUDE.md) of its own, without
  // R3V's part: offered at its end.
  let guideAsk = $state<{ root: string; theirs: boolean } | null>(null);
  async function offerGuide(root: string) {
    if (!nightly) return;
    try {
      const has = await api.AgentGuide(root);
      if (has !== "has") guideAsk = { root, theirs: has === "theirs" };
    } catch {
      // (offered again in the Agents tab)
    }
  }
  async function addGuide() {
    const root = guideAsk!.root;
    guideAsk = null;
    try {
      const written = await api.AddAgentGuide(root);
      toast(t("Added {files}: they go with the project's first version.", { files: written.join(", ") }), "ok", 8000);
    } catch (e) {
      toast(errorText(e), "error");
    }
  }

  // A folder R3V doesn't recognise (only documents, a new project): asked
  // how to keep it, then added as said.
  let kindAsk = $state<string | null>(null);
  async function share(folder: string, preset = "") {
    busy = "add";
    try {
      const got = preset ? await api.AddProjectAs(overview!.currentTeam, folder, preset)
        : await api.AddProjectToTeam(overview!.currentTeam, folder);
      kindAsk = null;
      firstShare = got.root;
      await reload();
      select(got);
      offerGuide(got.root);
    } catch (e) {
      if (!preset && unknownKind(e)) kindAsk = folder;
      else toast(errorText(e), "error", 9000);
    } finally {
      busy = "";
    }
  }


  async function toggleAutostart(on: boolean) {
    try {
      await api.SetAutostart(on);
      autostart = on;
    } catch (e) {
      toast(errorText(e), "error");
    }
  }

  type WatchEvent = { root: string; kind: string; author: string; labels: string[]; text: string; versions: { author: string; message: string }[] };

  onMount(() => {
    watchPreuploads();
    reload().then(() => {
      if (overview && overview.teams.length === 0) onboarding = true;
    });
    api.Autostart().then((on) => (autostart = on)).catch(() => {});
    loadProfile();
    api.Version().then((v) => (appVersion = v)).catch(() => {});
    api.Edition().then((e) => (edition = e)).catch(() => {});
    api.Channel().then((c) => (nightly = c.build === "nightly")).catch(() => {});
    checkUpdate();
    const offProgress = Events.On("progress", (ev: { data: Progress }) => onProgress(ev.data));
    const offUpdate = Events.On("update", (ev: { data: UpdateState }) => onUpdateState(ev.data));
    const offWatch = Events.On("team-watch", (ev: { data: WatchEvent }) => {
      const e = ev.data;
      const name = entries.find((p) => p.root === e.root)?.name ?? "";
      switch (e.kind) {
        case "new-versions":
          toast(`${name}: ${e.versions.map((v) => t("{author} saved “{message}”", { author: v.author, message: titleOf(v.message) })).join("\n")}`, "info", 8000);
          break;
      }
      if (e.root === selected.root) refreshKey++;
    });
    // A hosted team's people, roles or projects changed (R3V-Cloud).
    const offTeams = Events.On("teams", () => reload());
    const offLinks = Events.On("open-link", () => openLinks());
    openLinks(); // (started from a link)
    return () => {
      offProgress();
      offUpdate();
      offWatch();
      offTeams();
      offLinks();
    };
  });

  // The shell's keys (not while a dialog is open): Ctrl+\ the sidebar;
  // Ctrl+W closes the tab, Ctrl+Shift+T opens the last closed one again;
  // Ctrl+Tab / Ctrl+Shift+Tab (or Ctrl+PageDown / PageUp) the next and
  // previous tab, Ctrl+1…8 a tab, Ctrl+9 the last.
  let keysHelp = $state(false);
  function tabKeysDown(e: KeyboardEvent) {
    if (!(e.ctrlKey || e.metaKey) || e.altKey || !overview || onboarding || document.querySelector("[aria-modal='true']")) return;
    const k = e.key.toLowerCase();
    if (e.key === "/" || e.code === "Slash") { e.preventDefault(); keysHelp = true; return; }
    const at = tabItems.findIndex(activeTab);
    const go = (i: number) => { const x = tabItems[(i + tabItems.length) % tabItems.length]; if (x) goTab(x); };
    if (e.key === "\\" || e.code === "Backslash") fold(!folded);
    else if (k === "w" && !e.shiftKey) { if (at >= 0) closeTabOf(tabItems[at]); }
    else if (k === "t" && e.shiftKey) reopenTab();
    else if (e.key === "Tab" || e.key === "PageDown" || e.key === "PageUp") {
      if (!tabItems.length) return;
      go(at + (e.key === "PageUp" || (e.key === "Tab" && e.shiftKey) ? -1 : 1));
    } else if (/^[1-9]$/.test(e.key) && !e.shiftKey) {
      if (!tabItems.length) return;
      go(e.key === "9" ? tabItems.length - 1 : Math.min(+e.key - 1, tabItems.length - 1));
    } else return;
    e.preventDefault();
  }

  const statusText = (status: string) => ({ remote: t("not downloaded"), missing: t("folder not found") } as Record<string, string>)[status];
</script>

<svelte:window onfocus={reloadIfStale}
  onkeydown={(e) => {
    if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === "t" && overview && !onboarding
      && !e.repeat && !document.querySelector("[aria-modal='true']") && !(e.target as HTMLElement).closest?.("input, textarea, [contenteditable]")) {
      e.preventDefault();
      launching = !launching;
    }
    if (e.key === "Escape" && rowMenu) { e.preventDefault(); rowMenu = ""; }
    tabKeysDown(e);
    // F5, Ctrl+R: the project list (an open project refreshes itself), never the page.
    if (e.key === "F5" || ((e.ctrlKey || e.metaKey) && !e.altKey && e.key.toLowerCase() === "r")) {
      e.preventDefault();
      if (overview && !onboarding) reload();
    }
  }}
  onclick={(e) => {
    if (rowMenu && !(e.target as HTMLElement).closest(".row-menu")) rowMenu = "";
  }} />

{#if !overview || onboarding}
  <div class="bare-bar">{@render winControls()}</div>
{/if}
{#if !overview}
  <div class="splash" role="status" aria-label={t("Loading")}>
    <div class="splash-logo"><img src="/brand/r3v-icon.svg" alt="" /><img class="word" src="/brand/r3v-wordmark-on-dark.svg" alt="R3V" /></div>
    <div class="splash-band"></div>
  </div>
{:else if onboarding}
  <Onboarding {overview} {reload} onfinish={async (root, share) => {
    onboarding = false;
    if (root && share) {
      firstShare = root;
      offerGuide(root);
    }
    else if (root) justDownloaded = root;
    await reload();
    const done = root ? entries.find((p) => p.root === root) : undefined;
    if (done) select(done);
    else if (root) selected = { root };
  }} />
{:else}
  <div class="shell" class:folded class:sizing style:grid-template-columns="{folded ? 60 : sideW}px minmax(0, 1fr)">
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="side-split" style:left="{folded ? 60 : sideW}px" onpointerdown={sideDrag}></div>
    <aside>
      {#if folded}
      <!-- folded: the same rows as unfolded, so nothing moves when it folds -->
      <div class="aside-top folded-top">
        <button class="logo" onclick={() => (appSettings = true)} title={t("R3V settings")} aria-label={t("R3V settings")}>
          <img src="/brand/r3v-icon-small.svg" alt="" />
          {#if update || current?.keysUnreadable || overview.teamError}<span class="news" title={t("Open the sidebar to see what's new")}></span>{/if}
        </button>
      </div>
      {#if current}
        <div class="team-slot">
          <button class="team-mark" class:on={home}
            onclick={(e) => (e.ctrlKey || e.metaKey ? openTab(homeRef(teamId)) : goHome())}
            onmousedown={noAutoscroll} onauxclick={middle(() => openTab(homeRef(teamId)))} title={current.name} aria-label={current.name}><TeamMark team={current} size={36} /></button>
        </div>
      {/if}
      <div class="section-slot">
        <button class="ghost fold" onclick={() => fold(false)} title={t("Show the sidebar") + " (Ctrl+\\)"} aria-label={t("Show the sidebar")}>»</button>
      </div>
      <!-- folded: the projects as their icons -->
      <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
      <div class="folded-list" data-projects onclickcapture={(e) => { if (projDragged) { e.stopPropagation(); e.preventDefault(); } }}>
        {#each rows as p (p.root || p.id)}
          <div class="folded-row" data-key={orderOf(p)} class:lifted={dragging === orderOf(p)} animate:flip={slide(orderOf(p))}
            style:transform={dragging === orderOf(p) ? `translateY(${dragDy}px)` : undefined} onpointerdown={(e) => projDown(e, orderOf(p))}>
            <button class="folded-proj {p.status}" class:on={!home && selectedEntry === p} onclick={(e) => pick(p, e)}
              onmousedown={noAutoscroll} onauxclick={middle(() => openTab(projRef(p)))} title={p.name} aria-label={p.name}>
              <ProjectIcon {p} size={30} />
            </button>
          </div>
        {/each}
        <button class="folded-proj add-icon" onclick={addToTeam} disabled={busy === "add" || !current} title={t("Add project")} aria-label={t("Add project")}>+</button>
      </div>
      <!-- folded: who you are, as your picture -->
      {#if current?.memberName}
        <div class="user folded-user">
          {#if profile?.available}
            <button class="ghost who" onclick={() => (userSettings = true)} title={`${current.memberName} · ${t("User settings")}`}
              aria-label={t("User settings")}>
              <Avatar name={current.memberName} seed={current.memberId} color={profile.color} picture={profile.picture} size={28} />
            </button>
          {:else}
            <span class="avatar" title={current.memberName}>{([...current.memberName.trim()][0] ?? "?").toUpperCase()}</span>
          {/if}
        </div>
      {/if}
      {:else}
      <div class="aside-top">
      <button class="brand" onclick={() => (appSettings = true)} title={t("R3V settings")}>
        <img class="wordmark" src="/brand/r3v-wordmark-on-dark.svg" alt="R3V" />
        {#if edition}<span class="edition" title={t("A R3V build with extensions")}>{edition}</span>{/if}
      </button>
      <!-- the version, short (0.1.3, and Nightly); in full in its tooltip, copied on a click -->
      {#if appVersion}
        <button class="ghost version" onclick={copyVersion} title={t("R3V {version} — click to copy", { version: appVersion })}>
          v{appVersion.split("-")[0]}{#if appVersion.includes("-nightly")}<span class="channel">Nightly</span>{/if}
        </button>
      {/if}
      <button class="ghost fold" onclick={() => fold(true)} title={t("Hide the sidebar") + " (Ctrl+\\)"} aria-label={t("Hide the sidebar")}>«</button>
      </div>
      <div class="team-slot"><TeamMenu {overview} {reload} {home} onhome={() => goHome()} onhomebehind={() => openTab(homeRef(teamId))} onteam={switchTeam}
        onsettings={openTeamSettings} /></div>

      <div class="list">
        <div class="section row-h section-slot">
          <span>{t("Projects")}</span>
          {#if current}
            <button class="ghost tiny" class:spin={turning.on} onclick={reload} title={t("Check the team for new projects") + " (F5)"} aria-label={t("Check the team for new projects")}><svg class="ico-s" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-2.64-6.36L21 8"/><path d="M21 3v5h-5"/></svg></button>
          {/if}
        </div>
        <!-- (no line while the team is asked: the turning refresh says so,
             and the projects stay where they are) -->
        {#if current && overview.teamError}
          <div class="offline" title={overview.teamError}>● {t("Storage not reachable")}</div>
        {/if}
        <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
        <ul data-projects onclickcapture={(e) => { if (projDragged) { e.stopPropagation(); e.preventDefault(); } }}>
          {#each rows as p (p.root || p.id)}
            <li data-key={orderOf(p)} class:lifted={dragging === orderOf(p)} class:ws={p.workspace} class:has-ws={workspacesOf(p).length > 0}
              animate:flip={slide(orderOf(p))} style:transform={dragging === orderOf(p) ? `translateY(${dragDy}px)` : undefined}
              onpointerdown={(e) => { if (!p.workspace) projDown(e, orderOf(p)); }} oncontextmenu={(e) => openMenu(p, e)}>
              {@render row(p)}
            </li>
          {:else}
            <li class="empty faint">{t("No projects in this team yet.")}</li>
          {/each}
        </ul>
        <button class="add" onclick={addToTeam} disabled={busy === "add" || !current}>
          <span>+ {t("Add project")}</span>
          <span class="hint">{t("Select project folder")}</span>
        </button>
      </div>
      <!-- what needs you: R3V's update, the team's asks (kept below the
           projects, so they stay where they are folded or not) -->
      <div class="notices">
        {#if update}
          {@const u = update}
          <!-- a newer R3V, in a line: its version (What's new on a click), the button -->
          <div class="update" title={t("{app} {version} is available", { app: `R3V${edition ? ` ${edition}` : ""}`, version: u.version })}>
            <button class="link upd-v" onclick={() => openLink(u.pageUrl)} title={t("What's new")}>
              R3V {u.version.split("-")[0]}{#if u.version.includes("-nightly")}<span class="channel">Nightly</span>{/if}
            </button>
            {@render updateButton(u)}
            {#if !u.required}
              <button class="ghost x" title={t("Hide until the next version")} aria-label={t("Hide until the next version")}
                onclick={() => { remember(DISMISSED_KEY, u.version); update = null; }}>✕</button>
            {/if}
          </div>
          {#if updState.stage === "failed"}<p class="upd-error">{updState.error}</p>{/if}
        {/if}
        {#if current?.keysUnreadable}
          <p class="keys-warn">{t("This computer can't read the keys of “{team}” (R3V's settings came from another computer or Windows user). Enter them again in the team's settings (⚙).", { team: current.name })}</p>
        {/if}
        <TeamMenu {overview} {reload} part="notices" onhome={() => goHome()} onsettings={openTeamSettings} />
      </div>
      <!-- who you are in this team, and the app's settings -->
      <div class="user">
        {#if current?.memberName && profile?.available}
          <button class="ghost who" onclick={() => (userSettings = true)} title={t("User settings")}>
            <Avatar name={current.memberName} seed={current.memberId} color={profile.color} picture={profile.picture} />
            <span class="user-name">{current.memberName}</span>
          </button>
        {:else if current?.memberName}
          <span class="avatar" aria-hidden="true">{([...current.memberName.trim()][0] ?? "?").toUpperCase()}</span>
          <span class="user-name">{current.memberName}</span>
        {/if}
        <button class="ghost keys" onclick={() => (keysHelp = true)} title={t("Keyboard shortcuts") + " (Ctrl+/)"}
          aria-label={t("Keyboard shortcuts")}>⌨</button>
        <button class="ghost prefs" onclick={() => (appSettings = true)}>{t("Preferences")}</button>
      </div>
      {/if}

    </aside>

    <section class="content">
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="titlebar" ondblclick={(e) => { if (!(e.target as HTMLElement).closest(".tab, button")) win(() => Window.ToggleMaximise()); }}>
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div class="tabs" role="tablist" tabindex="-1" aria-label={t("Open projects")} bind:this={tabsEl}
          onclickcapture={(e) => { if (tabDragged) { e.stopPropagation(); e.preventDefault(); } }}>
          {#each tabItems as x (x.id)}
            {@const proj = x.p ?? x.snap}
            {@const team = teamOf(x.team)}
            {@const name = x.home ? team?.name ?? "" : proj?.name ?? ""}
            <!-- a middle click closes it, as × does -->
            <div class="tab" class:on={activeTab(x)} class:home-tab={x.home} onpointerdown={(e) => tabDown(e, x.id)}
              onmousedown={noAutoscroll} onauxclick={middle(() => closeTabOf(x))}>
              <!-- a team's home: its team's mark; a project: its icon -->
              <button class="tab-name" role="tab" aria-selected={activeTab(x)}
                title={x.home ? `${name} · ${t("Team home")}` : proj ? tabTitle(team?.name ?? "", proj) : name}
                onclick={() => goTab(x)}>
                {#if x.home}{#if team}<TeamMark {team} size={16} />{/if}{:else if proj}<ProjectIcon p={proj} size={16} />{/if}{name}
                {#if manyTeams && team && !x.home}<span class="tab-team"><TeamMark {team} size={16} /></span>{/if}
              </button>
              <button class="tab-x" onclick={() => closeTabOf(x)} aria-label={t("Close {name}", { name })}
                title={activeTab(x) ? t("Close the tab") + " (Ctrl+W)" : t("Close the tab")}>×</button>
            </div>
          {/each}
          <button class="tab-add" class:on={launching} onclick={() => (launching = !launching)} aria-label={t("Open a project or team…")}
            title={t("Open a project or team…") + " (Ctrl+T)"}>+</button>
        </div>
        {@render winControls()}
      </div>
      <div class="content-body">
      {#if current && (home || !selectedEntry)}
        <TeamHome {overview} team={current} projects={entries} open={isOpen} onopen={select} onadd={addToTeam}
          adding={busy === "add"} {reload} canMove agents={nightly} bind:section={homeSection} />
      {:else if selectedEntry && selectedEntry.status === "downloaded"}
        {#key selectedEntry.root}
          <ProjectView root={selectedEntry.root} {refreshKey} teams={overview.teams} onchanged={reload}
            onsettings={() => (settingsFor = selectedEntry ?? null)}
            entry={selectedEntry} onteamsettings={openTeamSettings}
            settings={settingsTab} onopenproject={openWorkspace}
            firstShare={firstShare === selectedEntry.root} onfirstshared={() => (firstShare = "")}
            downloaded={justDownloaded === selectedEntry.root} ondownloadseen={() => (justDownloaded = "")} />
        {/key}
      {:else if selectedEntry && selectedEntry.status === "remote"}
        {@const p = selectedEntry}
        <div class="placeholder">
          <div class="big" aria-hidden="true">☁</div>
          <h1>{p.name}</h1>
          <p class="muted">{t("This project is on {team} but not on this computer yet.", { team: current?.name ?? "" })}</p>
          {#if teamLocked}
            <p class="faint">{overview.teamError ? t("{team} can't be reached right now: download it once it's back.", { team: current?.name ?? "" })
              : t("Checking {team}…", { team: current?.name ?? "" })}</p>
          {:else if dlSize?.id === p.id && dlSize.size}
            {@const z = dlSize.size}
            <p class="size">{formatBytes(z.bytes)} · {tn(z.files, "{count} file", "{count} files", { count: z.files.toLocaleString() })}</p>
          {:else if dlSize?.id === p.id && !dlSize.error}
            <p class="size faint">{t("Checking the size…")}</p>
          {/if}
          <button class="primary" onclick={() => download(p)} disabled={!!busy || teamLocked}>
            {busy === p.id ? t("Downloading…") : `↓ ${t("Download")}`}
          </button>
          {#if busy === p.id && lastProgress}
            <div class="dl-progress"><ProgressBar p={lastProgress} /></div>
          {/if}
          <p class="faint small">{t("Into {folder}", { folder: downloadDir || t("a folder you choose") })} ·
            <button class="link" onclick={changeDownloadDir}>{t("change")}</button></p>
        </div>
      {:else if selectedEntry && selectedEntry.status === "missing"}
        {@const p = selectedEntry}
        <div class="placeholder">
          <div class="big" aria-hidden="true">⚠</div>
          <h1>{p.name}</h1>
          <p class="muted">{t("The project folder was moved or deleted:")}<br /><span class="mono">{p.root}</span></p>
          <div class="row center">
            <button class="primary" onclick={() => locate(p)}>{t("Locate folder…")}</button>
            <button onclick={() => download(p)} disabled={!!busy}>{t("Download again")}</button>
            <button class="ghost" onclick={() => forget(p)} title={t("R3V stops listing it; nothing is deleted")}>{t("Unlink folder")}</button>
          </div>
        </div>
      {:else}
        <div class="placeholder">
          <h1>{current?.name ?? ""}</h1>
          <p class="muted">{t("Pick a project on the left, or add one to share it with the team.")}</p>
        </div>
      {/if}
      </div>
    </section>
  </div>
{/if}

{#snippet winControls()}
  <div class="win">
    <button onclick={() => win(() => Window.Minimise())} aria-label={t("Minimize")} title={t("Minimize")}>
      <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M1 5.5h8" /></svg>
    </button>
    <button onclick={() => win(() => Window.ToggleMaximise())} aria-label={t("Maximize")} title={t("Maximize")}>
      <svg viewBox="0 0 10 10" aria-hidden="true"><rect x="1.5" y="1.5" width="7" height="7" /></svg>
    </button>
    <button class="close" onclick={() => win(() => Window.Close())} aria-label={t("Close")} title={t("Close (R3V keeps running in the tray)")}>
      <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M1.5 1.5l7 7M8.5 1.5l-7 7" /></svg>
    </button>
  </div>
{/snippet}

{#snippet updateActions(u: UpdateInfo)}
  <div class="update-a">
    {@render updateButton(u)}
    <button class="ghost" onclick={() => openLink(u.pageUrl)}>{t("What's new")}</button>
  </div>
  {#if updState.stage === "failed"}<p class="upd-error">{updState.error}</p>{/if}
{/snippet}

{#snippet updateButton(u: UpdateInfo)}
    {#if !u.installable}
      {#if u.downloadUrl}
        <button class="primary" onclick={() => openLink(u.downloadUrl)}
          title={t("Download the installer; run it to update (your projects and teams are kept)")}>{t("Download")}</button>
      {/if}
    {:else if updState.stage === "downloading" || (installWhenReady && updState.stage !== "failed")}
      <span class="upd-progress"><span style:width="{pct(updState)}%"></span></span>
      <span class="faint small">{updState.stage === "ready" ? t("Installing…") : t("Downloading {percent}%", { percent: pct(updState) })}</span>
    {:else if updState.stage === "ready"}
      <button class="primary" onclick={updateNow} disabled={anyBusy}
        title={anyBusy ? t("After the project's current upload or download") : t("R3V closes, updates and opens again (your projects and teams are kept)")}>
        {t("Restart to update")}</button>
    {:else}
      <button class="primary" onclick={updateNow} disabled={anyBusy}
        title={t("Downloads it, then R3V closes, updates and opens again")}>{t("Update now")}</button>
    {/if}
{/snippet}

<!-- a project's row in the sidebar (in its <li>: animate:flip wants that in the each) -->
{#snippet row(p: TeamProject)}
  {@const spaces = workspacesOf(p)}
    <button class="proj {p.status}" class:on={!home && selectedEntry === p} onclick={(e) => pick(p, e)}
      onmousedown={noAutoscroll} onauxclick={middle(() => openTab(projRef(p)))}
      class:locked={p.status === "remote" && teamLocked}
      title={p.status === "remote" ? t("On the team, not on this computer yet") : p.workspace ? t("An agent's workspace of {name}: {folder}", { name: p.name, folder: p.root }) : p.root}>
      <ProjectIcon {p} size={24} />
      <span class="text">
        <span class="name">{p.workspace ? `⑂ ${p.branchLabel || p.branch}` : p.name}</span>
        <span class="meta" class:busy={p.root && activity[p.root]}>
          {p.root && activity[p.root] ? progressShort(activity[p.root]) : statusText(p.status) ?? (p.workspace ? t("Workspace") : `⑂ ${p.branchLabel || p.branch}`)}
        </span>
      </span>
    </button>
    {#if p.root && preuploads[p.root]}<span class="pre-slot"><PreuploadIcon p={preuploads[p.root]} /></span>{/if}
    {#if spaces.length}
      <button class="ghost fold-ws" aria-expanded={!wsFolded[p.root]} onclick={() => foldWorkspaces(p)}
        title={wsFolded[p.root] ? t("Show its workspaces") : t("Hide its workspaces")}>
        <span class="caret" aria-hidden="true">{wsFolded[p.root] ? "▸" : "▾"}</span>{spaces.length}</button>
    {/if}
    {#if rowMenu === rowKey(p)}
      <div class="row-menu surface-menu" role="menu" style:left="{menuAt.left}px" style:top="{menuAt.top}px">
        {#if menuInfo && menuInfo.openable.length === 1}
          <button class="item" onclick={() => openIn(p, menuInfo!.openable[0])}>{t("Open in {tool}", { tool: toolName(menuInfo.tool) })}</button>
        {:else if menuInfo && menuInfo.openable.length > 1}
          <div class="sub" role="none" onmouseenter={showSub} onmouseleave={hideSub}>
            <button class="item has-sub" aria-expanded={openSub}>
              {t("Open in {tool}", { tool: toolName(menuInfo.tool) })}<span class="arrow">›</span>
            </button>
            {#if openSub}
              <div class="row-menu submenu surface-menu" role="menu" tabindex="-1" style:left="{subAt.left}px" style:top="{subAt.top}px"
                onmouseenter={() => clearTimeout(subTimer)} onmouseleave={hideSub}>
                {#each menuInfo.openable as rel}
                  <button class="item" onclick={() => openIn(p, rel)}>{openLabel(p, rel)}</button>
                {/each}
              </div>
            {/if}
          </div>
        {/if}
        {#if p.root && p.status !== "missing"}
          <button class="item" onclick={() => { rowMenu = ""; api.ShowFolder(p.root); }}>{t("Open folder")}</button>
        {/if}
        {#if !p.workspace}
          <div class="sep"></div>
          <button class="item" onclick={() => { rowMenu = ""; settingsFor = p; }}>{t("Settings…")}</button>
        {/if}
      </div>
    {/if}
{/snippet}


<Tooltip />
{#if launching && overview}
  <QuickLaunch teams={overview.teams} current={teamId} {recent} onpick={launch} onclose={() => (launching = false)} />
{/if}
{#if keysHelp}<KeysHelp onclose={() => (keysHelp = false)} />{/if}
{#if queue.open}
  <UploadQueue names={Object.fromEntries(entries.filter((p) => p.root).map((p) => [p.root, p.name]))} onclose={() => (queue.open = false)} />
{/if}

{#if appSettings}
  <AppSettings version={appVersion} {edition} {autostart} autoUpdate={updState.auto} {downloadDir} {update}
    checking={checkingNow} onautostart={toggleAutostart} onautoupdate={setAutoUpdate} oncheck={checkUpdateNow}
    ondownloaddir={changeDownloadDir} onupdate={(u) => (update = u)} onclose={() => (appSettings = false)} />
{/if}

{#if guideAsk}
  <ConfirmDialog title={guideAsk.theirs ? t("Add R3V's part to the project's guide for AI agents?") : t("Add a guide for AI agents?")}
    confirm={t("Add the guide")} cancel={t("Not now")}
    text={guideAsk.theirs
      ? t("This project has its own AGENTS.md (or CLAUDE.md). R3V's part goes at its end, the rest left as it is: that the project is versioned with R3V and how to work in it (commit, settle conflicts, work beside other agents).")
      : t("AGENTS.md (with a line in CLAUDE.md that reads it) tells AI agents, Claude Code, Codex and others, that this project is versioned with R3V and how to work in it: commit, settle conflicts, work beside other agents. It's committed with the project.")}
    onconfirm={addGuide} onclose={() => (guideAsk = null)} />
{/if}

{#if kindAsk}
  <KindDialog folder={kindAsk} code busy={busy === "add"} onpick={(preset) => share(kindAsk!, preset)}
    onclose={() => (kindAsk = null)} />
{/if}

{#if userSettings}
  <UserSettings team={current} onclose={() => (userSettings = false)}
    onchanged={async (p) => { profile = p; await reload(); refreshKey++; }} />
{/if}

{#snippet projectSettings(p: TeamProject, inline: boolean)}
  <ProjectSettings {p} team={current} {inline} teams={overview?.teams ?? []} progress={p.root ? activity[p.root] ?? null : null}
    onmoved={!nightly ? undefined : async (teamId, copied) => { // (Nightly, while it's new)
      settingsFor = null;
      toast(copied ? t("Copied to the other team") : t("Moved: the project is in the other team now"), "ok", 8000);
      if (copied) return;
      const root = p.root;
      await api.SelectTeam(teamId);
      await reload();
      const moved = entries.find((e) => e.root === root);
      if (moved) select(moved);
    }}
    onclose={() => (settingsFor = null)}
    onrenamed={async () => { const key = rowKey(p); await reload(); refreshKey++; if (settingsFor) settingsFor = entries.find((e) => rowKey(e) === key) ?? null; }}
    oncheck={() => { settingsFor = null; checking = p; }}
    ondelete={() => { settingsFor = null; deleteWord = ""; confirmDelete = p; }}
    onunlink={() => { settingsFor = null; leaving = { roots: [p.root], go: () => forget(p) }; }}
    onlocate={() => { settingsFor = null; locate(p); }} />
{/snippet}
{#snippet settingsTab()}{#if selectedEntry}{@render projectSettings(selectedEntry, true)}{/if}{/snippet}

{#if teamSettingsFor && overview}
  {@const tm = overview.teams.find((x) => x.id === teamSettingsFor!.id) ?? teamSettingsFor}
  <TeamSettings team={tm} author={overview.author} {reload} onclose={() => (teamSettingsFor = null)}
    teams={overview.teams} canMove
    offline={tm.id === overview.currentTeam && !!overview.teamError}
    roots={tm.id === overview.currentTeam ? overview.projects.filter((p) => p.root && p.status === "downloaded").map((p) => p.root) : []} />
{/if}

{#if settingsFor}
  {@render projectSettings(settingsFor, false)}
{/if}

{#if checking}
  <VerifyDialog root={checking.root} name={checking.name} onclose={() => (checking = null)} />
{/if}


{#if confirmDelete}
  {@const p = confirmDelete}
  <Modal title={t("Delete “{name}” from the team?", { name: p.name })} onclose={() => (confirmDelete = null)}>
    <p>{t(p.root ? "This removes the project and all its versions from {team}, for everyone in the team. Copies already on someone's computer are not touched — yours stays in its folder."
      : "This removes the project and all its versions from {team}, for everyone in the team. Copies already on someone's computer are not touched.", { team: current?.name ?? "" })}</p>
    <label for="delete-word">{t("Type {name} to confirm", { name: p.name })}</label>
    <input id="delete-word" class="confirm-input" bind:value={deleteWord} autocomplete="off"
      onkeydown={(e) => { if (e.key === "Enter" && deleteWord.trim() === p.name) leaveThenDelete(p); }} />
    {#snippet footer()}
      <button onclick={() => (confirmDelete = null)}>{t("Cancel")}</button>
      <button class="danger" disabled={deleteWord.trim() !== p.name} onclick={() => leaveThenDelete(p)}>{t("Delete")}</button>
    {/snippet}
  </Modal>
{/if}

{#if leaving}
  <!-- go is taken before leaving is cleared: an {@const} of it would clear too. -->
  <KeptSamples roots={leaving.roots} oncancel={() => (leaving = null)}
    onproceed={() => { const go = leaving?.go; leaving = null; go?.(); }} />
{/if}

{#if update?.required}
  {@const u = update}
  <div class="must-update" role="dialog" aria-modal="true" aria-label={t("Update R3V")}>
    <div class="must-card">
      <h2>{t("Update R3V to keep going")}</h2>
      <p class="muted">{t("This version of R3V can no longer work with your team's projects: they need R3V {version}. Updating takes a minute, and keeps your projects and teams.", { version: u.version })}</p>
      {@render updateActions(u)}
    </div>
  </div>
{/if}

<Toasts />
{#if nightly}<StyleLab />{/if}

<style>
  .shell { position: relative; display: grid; grid-template-columns: 256px 1fr; height: 100%; }
  .side-split { position: absolute; top: 0; bottom: 0; width: 7px; margin-left: -4px; z-index: var(--z-sticky); cursor: col-resize; touch-action: none; }
  .side-split:hover, .shell.sizing .side-split { background: linear-gradient(to right, transparent 3px, var(--accent) 3px, var(--accent) 4px, transparent 4px); }
  .shell.sizing { cursor: col-resize; user-select: none; }
  .shell.folded { grid-template-columns: 60px 1fr; }
  /* Folded or not, the rows above the projects are as high, and a project
     row too: the projects stay where they are when the sidebar folds. */
  aside { --top-h: 40px; --team-h: 52px; --section-h: 34px; --row-h: 40px; }
  .aside-top { height: var(--top-h); flex: none; }
  .team-slot { min-height: var(--team-h); flex: none; display: flex; align-items: center; }
  .team-slot > :global(*) { flex: 1; min-width: 0; }
  .team-slot :global(.current) { height: var(--team-h); box-sizing: border-box; padding: var(--sp-6) 0; }
  .section-slot { height: var(--section-h); box-sizing: border-box; flex: none; }
  .user { display: flex; align-items: center; gap: var(--sp-8); margin: var(--sp-8) calc(var(--sp-10) * -1) calc(var(--sp-12) * -1);
    padding: var(--sp-10) var(--sp-14); border-top: var(--border-width) solid var(--line); }
  .user .avatar { width: 24px; height: 24px; border-radius: 50%; display: flex; align-items: center; justify-content: center;
    font-size: var(--fs-xs); font-weight: var(--fw-semibold); background: var(--panel-2); color: var(--text); }
  .who { flex: 1; min-width: 0; display: flex; align-items: center; gap: var(--sp-8); padding: var(--sp-2) var(--sp-4);
    margin-left: calc(var(--sp-4) * -1); text-align: left; color: var(--text); }
  .user-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: var(--fs-md); }
  .prefs { margin-left: auto; padding: var(--sp-2) var(--sp-6); font-size: var(--fs-sm); color: var(--faint); }
  .keys { margin-left: auto; padding: var(--sp-2) var(--sp-6); font-size: var(--fs-md); color: var(--faint); }
  .keys + .prefs { margin-left: 0; }
  .shell.folded aside { padding-left: var(--sp-6); padding-right: var(--sp-6); }
  .folded-user { justify-content: center; margin-left: calc(var(--sp-6) * -1); margin-right: calc(var(--sp-6) * -1); padding: var(--sp-10) 0; }
  .folded-user .who { flex: none; margin: 0; padding: var(--sp-2); }
  .aside-top { display: flex; align-items: center; gap: var(--sp-4); }
  .aside-top .brand { flex: 1; min-width: 0; }
  .aside-top .version { margin-bottom: var(--sp-8); }
  .fold { flex: none; padding: var(--sp-2) var(--sp-8); color: var(--faint); font-size: var(--fs-lg); line-height: 1; margin-bottom: var(--sp-8); }
  .fold:hover:not(:disabled) { color: var(--text); }
  .folded-top { justify-content: center; }
  .shell.folded .team-slot, .shell.folded .section-slot { justify-content: center; }
  .shell.folded .section-slot { display: flex; align-items: center; }
  .shell.folded .section-slot .fold { margin: 0; }
  .team-mark { flex: none !important; width: 36px; height: 36px; padding: 0; border-radius: var(--radius); display: flex; align-items: center; justify-content: center;
    background: transparent; border-color: transparent; }
  .team-mark:hover:not(:disabled) { border-color: var(--line-strong); background: transparent; }
  .team-mark.on { border-color: var(--accent); }
  .folded-list { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; align-items: center;
    padding: 0; margin: 0 calc(var(--sp-6) * -1); }
  .folded-row { height: var(--row-h); flex: none; display: flex; align-items: center; justify-content: center; }
  .folded-proj { padding: var(--sp-2); border: var(--border-width) solid transparent; border-radius: var(--radius-lg); background: transparent; line-height: 0; }
  .folded-proj:hover:not(:disabled) { background: var(--panel); border-color: transparent; }
  .folded-proj.on { background: var(--accent-bg); border-color: var(--accent-line); }
  .folded-proj.remote { opacity: .45; }
  .add-icon { flex: none; width: 36px; height: 36px; margin-top: var(--sp-4); line-height: 1; font-size: var(--fs-lg); color: var(--muted); border-style: dashed; border-color: var(--line); }
  /* The project dragged: lifted over the others, where the pointer is; let
     go, it settles into its place. */
  li[data-key], .folded-row { touch-action: none; transition: transform .15s ease-out; }
  .lifted { z-index: var(--z-sticky); transition: none; cursor: grabbing; }
  /* (solid under a dimmed row: the rows it passes don't show through) */
  li.lifted, .folded-row.lifted { background: var(--bg-sunken); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop); }
  .lifted .proj, .lifted .folded-proj { background: var(--panel-2); }
  @media (prefers-reduced-motion: reduce) { li[data-key], .folded-row { transition: none; } }
  .logo { border: none; background: transparent; padding: var(--sp-6); border-radius: var(--radius); }
  .logo { position: relative; }
  .logo img { width: 22px; height: 22px; display: block; }
  .logo .news { position: absolute; top: 2px; right: 2px; width: 8px; height: 8px; border-radius: 50%; background: var(--warn);
    box-shadow: 0 0 0 2px var(--bg-sunken); }
  aside { background: var(--bg-sunken); border-right: var(--border-width) solid var(--line); display: flex; flex-direction: column; padding: var(--sp-12) var(--sp-10); min-height: 0; }
  .brand { display: flex; align-items: center; gap: var(--sp-8); font-weight: var(--fw-bold); font-size: var(--fs-lg); padding: var(--sp-4) var(--sp-8); margin: calc(var(--sp-2) * -1) 0 var(--sp-8);
    background: none; border: 0; border-radius: var(--radius); color: var(--text); text-align: left; cursor: pointer; width: 100%; }
  .brand:hover { background: var(--panel-2); }
  .brand img { width: 24px; height: 24px; }
  .brand img.wordmark { width: auto; height: 18px; }
  .list { flex: 1; overflow: auto; min-height: 0; }
  .section { font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); padding: var(--sp-8) var(--sp-8) var(--sp-4); }
  .row-h { display: flex; align-items: center; justify-content: space-between; }
  .tiny { padding: 0 var(--sp-6); font-size: var(--fs-md); line-height: 18px; color: var(--faint); }
  .tiny:hover { color: var(--text); }
  .ico-s { width: 13px; height: 13px; display: block; }
  .spin .ico-s { animation: spin .8s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .meta.busy { color: var(--accent); }
  .size { margin: 0 0 var(--sp-12); font-size: var(--fs-md); }
  .dl-progress { width: 360px; max-width: 100%; margin: var(--sp-10) auto 0; display: flex; text-align: left; }
  ul { list-style: none; margin: 0; padding: 0; }
  li { display: flex; align-items: center; position: relative; }
  .proj { flex: 1; min-width: 0; height: var(--row-h); box-sizing: border-box; display: flex; align-items: center; gap: var(--sp-10); border: none; background: transparent; padding: 0 var(--sp-28) 0 var(--sp-10); border-radius: var(--radius-lg); text-align: left; }
  .proj:hover { background: var(--panel); }
  /* an agent's workspace: under its project, a step in */
  li.ws .proj { padding-left: var(--sp-28); }
  li.has-ws .proj { padding-right: calc(var(--sp-28) + var(--sp-16)); }
  .proj.on { background: var(--accent-bg); }
  .icon { width: 16px; text-align: center; color: var(--accent); }
  .text { display: flex; flex-direction: column; min-width: 0; }
  .name { font-weight: var(--fw-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { font-size: var(--fs-sm); color: var(--faint); }
  /* Not on this computer: dimmed, with a cloud icon (not colour alone). */
  .proj.remote .name { color: var(--muted); font-weight: var(--fw-medium); }
  .proj.remote .icon { color: var(--faint); }
  .proj.locked { opacity: .6; }
  .proj.missing .icon, .proj.missing .meta { color: var(--warn); }
  /* a project's workspaces, folded or not (the count) */
  .fold-ws { position: absolute; top: 50%; right: 4px; transform: translateY(-50%); display: inline-flex; align-items: center; gap: var(--sp-2);
    padding: 0 var(--sp-6); line-height: 20px; font-size: var(--fs-sm); color: var(--faint); border-radius: var(--radius);
    font-variant-numeric: tabular-nums; }
  .fold-ws .caret { font-size: var(--fs-xs); }
  .fold-ws:hover { background: var(--hover); color: var(--text); }
  /* (beside the project's button, not in it: a button of its own) */
  .pre-slot { position: absolute; top: 9px; right: 28px; }
  .row-menu {
    position: fixed; z-index: var(--z-menu); min-width: 220px; padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop);
  }
  .row-menu .item {
    display: flex; flex-direction: column; align-items: flex-start; gap: 1px; width: 100%; border: none;
    background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left;
  }
  .row-menu .item:hover { background: var(--hover); }
  .row-menu .sep { height: 1px; background: var(--line); margin: var(--sp-4) var(--sp-2); }
  .row-menu .sub { position: relative; }
  .row-menu .has-sub { flex-direction: row; justify-content: space-between; align-items: center; }
  .row-menu .arrow { color: var(--faint); }
  .row-menu.submenu { position: fixed; right: auto; min-width: 200px; max-width: 340px; z-index: var(--z-submenu); }
  .row-menu.submenu .item { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: block; }
  .row-menu .has-sub[aria-expanded="true"] { background: var(--hover); }
  .danger-text { color: var(--danger); }
  .confirm-input { width: 100%; margin-top: var(--sp-6); }
  .empty { padding: var(--sp-6) var(--sp-10); font-size: var(--fs-md); }
  .offline { font-size: var(--fs-sm); color: var(--danger); padding: 0 var(--sp-8) var(--sp-4); }
  .add { width: 100%; margin: var(--sp-8) 0 var(--sp-4); display: flex; flex-direction: column; align-items: center; gap: 0; padding: var(--sp-4) var(--sp-10); line-height: 1.3; }
  .add .hint { font-size: var(--fs-xs); color: var(--faint); font-weight: 400; }
  .pad { padding: 0 var(--sp-8); }
  .link { border: none; background: none; color: var(--muted); text-decoration: underline; padding: 0; font-size: var(--fs-md); text-align: left; }
  .version { margin-left: auto; display: inline-flex; align-items: center; gap: var(--sp-4); padding: var(--sp-2) var(--sp-4);
    font-size: var(--fs-xs); font-weight: 400; color: var(--faint); white-space: nowrap; }
  .version:hover { color: var(--muted); }
  .channel { padding: 0 var(--sp-6); border-radius: var(--radius-pill); background: var(--accent-soft); color: var(--accent);
    font-size: var(--fs-2xs); font-weight: var(--fw-semibold); letter-spacing: .04em; text-transform: uppercase; line-height: 1.5; }
  .edition { font-size: var(--fs-2xs); font-weight: var(--fw-bold); letter-spacing: .06em; text-transform: uppercase; padding: 1px var(--sp-6);
    border-radius: var(--radius); color: var(--accent-ink); background: var(--accent); }
  .notices { flex: none; display: flex; flex-direction: column; gap: var(--sp-6); }
  .update { display: flex; align-items: center; gap: var(--sp-6); margin: 0; padding: var(--sp-4) var(--sp-4) var(--sp-4) var(--sp-10);
    border-radius: var(--radius-lg); background: var(--accent-bg); border: var(--border-width) solid var(--accent-line); font-size: var(--fs-sm); }
  .upd-v { flex: 1; min-width: 0; display: inline-flex; align-items: center; gap: var(--sp-4); color: var(--accent); font-weight: var(--fw-semibold);
    font-size: var(--fs-sm); text-decoration: none; white-space: nowrap; overflow: hidden; }
  .update :global(button.primary) { flex: none; padding: var(--sp-2) var(--sp-10); font-size: var(--fs-sm); }
  .update .upd-progress { flex: 0 0 60px; }
  .update .small { font-size: var(--fs-xs); white-space: nowrap; }
  .update-a { display: flex; gap: var(--sp-6); margin-top: var(--sp-6); align-items: center; }
  .update-a button { padding: var(--sp-4) var(--sp-10); font-size: var(--fs-md); }
  .upd-progress { flex: 1; height: 5px; border-radius: var(--radius-xs); background: var(--accent-line); overflow: hidden; }
  .upd-progress span { display: block; height: 100%; background: var(--accent); transition: width .2s; }
  .upd-error { margin: var(--sp-6) 0 0; color: var(--danger); font-size: var(--fs-sm); }
  .must-update { position: fixed; inset: 0; z-index: var(--z-blocking); display: flex; align-items: center; justify-content: center;
    background: var(--scrim-strong); backdrop-filter: var(--scrim-filter); }
  .must-card { width: 440px; max-width: calc(100vw - 48px); padding: var(--sp-24) var(--sp-24); border-radius: var(--radius-xl);
    background: var(--panel-2); border: var(--border-width) solid var(--line); }
  .must-card h2 { margin: 0 0 var(--sp-8); font-size: var(--fs-xl); }
  .must-card .update-a button { padding: var(--sp-6) var(--sp-14); font-size: var(--fs-base); }
  .x { padding: 0 var(--sp-4); line-height: 16px; color: var(--muted); }
  .small { font-size: var(--fs-md); }
  .content { min-width: 0; overflow: hidden; display: flex; flex-direction: column; }
  .content-body { flex: 1; min-height: 0; overflow: hidden; }
  /* The title bar (the window has no Windows one): open projects as tabs,
     and the window's buttons. Empty space drags the window. */
  .titlebar, .aside-top { --wails-draggable: drag; }
  .titlebar button, .titlebar .tab, .aside-top button { --wails-draggable: no-drag; }
  .titlebar { display: flex; align-items: flex-end; height: 40px; flex: none; background: var(--bg-sunken);
    border-bottom: var(--border-width) solid var(--line); }
  .tabs { flex: 1; min-width: 0; display: flex; align-items: flex-end; gap: var(--sp-2); padding: 0 var(--sp-8); height: 100%; }
  .tab { display: flex; align-items: center; min-width: 0; max-width: 220px; height: 32px; border: var(--border-width) solid transparent;
    border-bottom: none; border-radius: var(--radius) var(--radius) 0 0; color: var(--muted); }
  .tab:hover { background: var(--panel); }
  .tab.on { background: var(--bg); border-color: var(--line); color: var(--text); margin-bottom: calc(var(--border-width) * -1); height: calc(32px + var(--border-width)); }
  .tab-name { flex: 1; min-width: 0; display: flex; align-items: center; gap: var(--sp-6); border: none; background: transparent;
    padding: 0 var(--sp-4) 0 var(--sp-12); color: inherit; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: var(--fs-md); }
  .tab-name:hover:not(:disabled) { background: transparent; }
  /* Which team a tab is on, once tabs of more than one are open. */
  .tab-team { flex: none; margin-left: auto; display: inline-flex; }
  .tab-x { border: none; background: transparent; padding: 0 var(--sp-8); color: var(--faint); font-size: var(--fs-lg); line-height: 1; visibility: hidden; }
  .tab:hover .tab-x, .tab.on .tab-x { visibility: visible; }
  .tab-x:hover:not(:disabled) { background: transparent; color: var(--text); }
  .tab-add { align-self: center; border: none; background: transparent; padding: 0 var(--sp-10); font-size: var(--fs-lg); color: var(--muted); }
  .tab-add.on { color: var(--text); }
  .tab { user-select: none; }
  .win { display: flex; align-self: stretch; }
  .win button { width: 46px; border: none; border-radius: 0; background: transparent; padding: 0; display: flex;
    align-items: center; justify-content: center; color: var(--muted); }
  .win button:hover:not(:disabled) { background: var(--hover); color: var(--text); }
  .win button.close:hover:not(:disabled) { background: var(--danger); color: var(--text); }
  .win svg { width: 10px; height: 10px; fill: none; stroke: currentColor; stroke-width: 1; }
  .bare-bar { position: fixed; top: 0; left: 0; right: 0; height: 32px; display: flex; justify-content: flex-end;
    z-index: var(--z-menu); --wails-draggable: drag; }
  .bare-bar button { --wails-draggable: no-drag; }
  .aside-top { margin: calc(var(--sp-12) * -1) calc(var(--sp-10) * -1) 0; padding: var(--sp-8) var(--sp-10) 0; }
  .placeholder { height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; padding: var(--sp-40); }
  .placeholder h1 { margin: var(--sp-6) 0; }
  .big { font-size: 44px; color: var(--faint); }
  .center { justify-content: center; }
  .keys-warn { margin: 0; padding: var(--sp-8) var(--sp-10); border-radius: var(--radius); font-size: var(--fs-sm); line-height: 1.4;
    background: var(--warn-bg); color: var(--warn); }
</style>
