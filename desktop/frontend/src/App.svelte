<script lang="ts">
  import { onMount } from "svelte";
  import { Events, Window } from "@wailsio/runtime";
  import { api, errorText, formatBytes, progressShort, type Overview, type Progress, type ProjectInfo, type TeamProject, type TeamSummary, type Profile } from "./lib/api";
  import type { DownloadSize, UpdateInfo, UpdateState } from "../bindings/github.com/nonlabhq/r3v/desktop/models";
  import ProgressBar from "./lib/ProgressBar.svelte";
  import { toast } from "./lib/notify.svelte";
  import ProjectView from "./lib/ProjectView.svelte";
  import NewTab from "./lib/NewTab.svelte";
  import { closeTab, isNew as isNewTab, loadTabs, moveTab, oldTabs, pruneTabs, refreshSnaps, renameTab, snapOf, tabId, teamsOf, toSave,
    type TabRef, type TabSnap } from "./lib/tabs";
  import { cssColor, initial, pickFor } from "./lib/palette";
  import ProjectIcon from "./lib/ProjectIcon.svelte";
  import { untrack } from "svelte";
  import { spinner } from "./lib/spin.svelte";
  import KeptSamples from "./lib/KeptSamples.svelte";
  import Onboarding from "./lib/Onboarding.svelte";
  import TeamMenu from "./lib/TeamMenu.svelte";
  import TeamSettings from "./lib/TeamSettings.svelte";
  import VerifyDialog from "./lib/VerifyDialog.svelte";
  import Modal from "./lib/Modal.svelte";
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
  import Avatar from "./lib/Avatar.svelte";
  import UserSettings from "./lib/UserSettings.svelte";


  let overview = $state<Overview | null>(null);
  let onboarding = $state(false);
  // Selected sidebar entry: a project folder, or a team project not here yet.
  let selected = $state<{ root?: string; id?: string }>({});
  let refreshKey = $state(0);
  let busy = $state("");
  let autostart = $state(false);
  let rowMenu = $state(""); // key of the project whose ⋯ menu is open
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
  // The ⋯ menu's "Open in" (read when the menu opens).
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
  function toggleMenu(p: TeamProject) {
    const key = rowKey(p);
    rowMenu = rowMenu === key ? "" : key;
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
  // Pinned projects come first in the list (this computer only).
  const PINNED_KEY = "r3v.pinned";
  let pinned = $state<string[]>((() => {
    try { const v = JSON.parse(localStorage.getItem(PINNED_KEY) ?? "[]"); return Array.isArray(v) ? v.filter((k) => typeof k === "string") : []; } catch { return []; }
  })());
  function togglePin(p: TeamProject) {
    const key = rowKey(p);
    pinned = pinned.includes(key) ? pinned.filter((k) => k !== key) : [...pinned, key];
    try { localStorage.setItem(PINNED_KEY, JSON.stringify(pinned)); } catch { /* not remembered */ }
  }
  const DOWNLOAD_DIR_KEY = "r3v.downloadDir";
  const remember = (k: string, v: string) => { try { localStorage.setItem(k, v); } catch { /* not persisted */ } };
  const recall = (k: string) => { try { return localStorage.getItem(k) ?? ""; } catch { return ""; } };

  let reloading = $state(false);
  const turning = spinner(() => reloading);
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
  // Every project is a team's (0.9: no projects kept on this computer only).
  let entries = $derived.by(() => {
    const list = overview?.projects ?? [];
    const pin = (p: TeamProject) => (pinned.includes(rowKey(p)) ? 0 : 1);
    return [...list].sort((a, b) => pin(a) - pin(b)); // stable: pinned first, the rest as they come
  });
  let selectedEntry = $derived(
    entries.find((p) => (selected.root ? p.root === selected.root : !!selected.id && p.id === selected.id && !p.root)));

  function select(p: TeamProject) {
    selected = p.root ? { root: p.root } : { id: p.id };
    if (p.root) remember(SELECTED_KEY, p.root);
    blank = "";
    openTab(p);
  }

  // Projects open as tabs at the top: one list for every team (remembered;
  // drag one to move it). The sidebar opens one of its team's, or goes to it
  // when it is open already; a tab of another team switches the sidebar to
  // that team first ("the sidebar follows the tab"). A new tab ("new:…",
  // Ctrl+T or +) picks a team and a project to open in it; new tabs aren't
  // remembered.
  const TABS_KEY = "r3v.tabs";
  const readJSON = (k: string): unknown => { try { return JSON.parse(localStorage.getItem(k) ?? "null"); } catch { return null; } };
  const savedTabs = readJSON(TABS_KEY);
  let tabs = $state<TabRef[]>(loadTabs(savedTabs, []));
  // Before one list for all, each team had its own: the first team shown
  // starts the list (the others' old lists are left as they were).
  let migrateTabs = savedTabs === null;
  let blank = $state(""); // the new tab shown ("" a project's)
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
  type Tab = { id: string; key: string; team: string; p?: TeamProject; snap?: TabSnap };
  let tabItems = $derived(tabs.map((x): Tab | null => {
    const id = tabId(x);
    if (isNewTab(x.key)) return { id, key: x.key, team: "" };
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
  const activeTab = (x: Tab) => (x.p ? !blank && !!selectedEntry && rowKey(selectedEntry) === x.key : !x.team && blank === x.key);
  function saveTabs() { remember(TABS_KEY, JSON.stringify(toSave(tabs))); }
  const isOpen = (p: TeamProject) => tabs.some((x) => x.team === teamId && x.key === rowKey(p));
  function openTab(p: TeamProject) {
    tabsClosed = false;
    if (!teamId || isOpen(p)) return;
    tabs = [...tabs, { team: teamId, key: rowKey(p), p: snapOf(p) }];
    saveTabs();
  }
  // A project's key changed (downloaded, found elsewhere, unlinked): its tab
  // stays where it was.
  function renamed(from: string, to: string) {
    tabs = renameTab(tabs, teamId, from, to);
    saveTabs();
  }
  function newTab() {
    const key = `new:${Date.now()}`;
    tabs = [...tabs, { team: "", key }];
    blank = key;
    tabsClosed = false;
  }
  // From a new tab: the project takes its place (or its own tab, if open).
  function openHere(p: TeamProject) {
    const here = blank;
    tabs = isOpen(p) ? tabs.filter((x) => x.key !== here)
      : tabs.map((x) => (x.key === here ? { team: teamId, key: rowKey(p), p: snapOf(p) } : x));
    saveTabs();
    select(p);
  }
  // Another team's tab: that team is shown (as the team menu does), then its
  // project, once the team lists it.
  async function goTab(x: Tab) {
    if (x.p) return select(x.p);
    if (!x.team) { blank = x.key; return; }
    switching = true;
    try {
      await api.SelectTeam(x.team);
      await reload();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      switching = false;
    }
    const p = teamId === x.team ? entries.find((e) => rowKey(e) === x.key) : undefined;
    if (p) select(p);
  }
  // Tabs closed, newest last: Ctrl+Shift+T opens the last one again (on its
  // team, while that team is still here).
  let closedTabs: TabRef[] = [];
  function reopenTab() {
    const x = closedTabs.pop();
    if (!x) return;
    if (x.team === teamId) {
      const p = entries.find((e) => rowKey(e) === x.key);
      if (p) select(p);
    } else if (teamOf(x.team) && x.p) {
      if (!tabs.some((y) => tabId(y) === tabId(x))) { tabs = [...tabs, x]; saveTabs(); }
      goTab({ id: tabId(x), key: x.key, team: x.team, snap: x.p });
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
    else if (n) blank = n.key;
    else { selected = {}; blank = ""; tabsClosed = true; }
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
  // The sidebar folds to the logo and a button to open it again (remembered).
  const SIDEBAR_KEY = "r3v.sidebar";
  let folded = $state(recall(SIDEBAR_KEY) === "folded");
  function fold(on: boolean) { folded = on; remember(SIDEBAR_KEY, on ? "folded" : ""); }
  // The window's own controls (no Windows title bar): in the browser
  // (server mode) they do nothing.
  const win = (f: () => Promise<unknown>) => f().catch(() => {});

  // Keep a valid selection when the team or the list changes.
  $effect(() => {
    if (!overview || selectedEntry || tabsClosed || blank || switching) return;
    const last = recall(SELECTED_KEY);
    const pick = untrack(() => tabItems).find((x) => x.p)?.p ?? entries.find((p) => p.root && p.root === last) ??
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

  async function share(folder: string) {
    busy = "add";
    try {
      const got = await api.AddProjectToTeam(overview!.currentTeam, folder);
      firstShare = got.root;
      await reload();
      select(got);
    } catch (e) {
      toast(errorText(e), "error", 9000);
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
          toast(`${name}: ${e.versions.map((v) => t("{author} saved “{message}”", { author: v.author, message: v.message })).join("\n")}`, "info", 8000);
          break;
      }
      if (e.root === selected.root) refreshKey++;
    });
    // A hosted team's people, roles or projects changed (R3V-Cloud).
    const offTeams = Events.On("teams", () => reload());
    return () => {
      offProgress();
      offUpdate();
      offWatch();
      offTeams();
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
      newTab();
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
    if (rowMenu && !(e.target as HTMLElement).closest(".row-menu, .more")) rowMenu = "";
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
    if (root && share) firstShare = root;
    else if (root) justDownloaded = root;
    await reload();
    const done = root ? entries.find((p) => p.root === root) : undefined;
    if (done) select(done);
    else if (root) selected = { root };
  }} />
{:else}
  <div class="shell" class:folded>
    <aside>
      {#if folded}
      <div class="aside-top folded-top">
        <button class="logo" onclick={() => (appSettings = true)} title={t("R3V settings")} aria-label={t("R3V settings")}>
          <img src="/brand/r3v-icon-small.svg" alt="" />
          {#if update || current?.keysUnreadable || overview.teamError}<span class="news" title={t("Open the sidebar to see what's new")}></span>{/if}
        </button>
        <button class="ghost fold" onclick={() => fold(false)} title={t("Show the sidebar") + " (Ctrl+\\)"} aria-label={t("Show the sidebar")}>»</button>
      </div>
      <!-- folded: the projects as their icons -->
      <div class="folded-list">
        {#each entries as p (p.root || p.id)}
          <button class="folded-proj" class:on={!blank && selectedEntry === p} onclick={() => select(p)} title={p.name} aria-label={p.name}>
            <ProjectIcon {p} size={28} />
          </button>
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
        <img src="/brand/r3v-icon-small.svg" alt="" /><img class="wordmark" src="/brand/r3v-wordmark-on-dark.svg" alt="R3V" />
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
      {#if update}
        {@const u = update}
        <div class="update">
          <div class="update-h">
            <span>{t("{app} {version} is available", { app: `R3V${edition ? ` ${edition}` : ""}`, version: u.version })}</span>
            {#if !u.required}
              <button class="ghost x" title={t("Hide until the next version")}
                onclick={() => { remember(DISMISSED_KEY, u.version); update = null; }}>✕</button>
            {/if}
          </div>
          {@render updateActions(u)}
        </div>
      {/if}
      <TeamMenu {overview} {reload} bind:settingsFor={teamSettingsFor} />
      {#if current?.keysUnreadable}
        <p class="keys-warn">{t("This computer can't read the keys of “{team}” (R3V's settings came from another computer or Windows user). Enter them again in the team's settings (⚙).", { team: current.name })}</p>
      {/if}

      <div class="list">
        <div class="section row-h">
          <span>{t("Projects")}</span>
          {#if current}
            <button class="ghost tiny" class:spin={turning.on} onclick={reload} title={t("Check the team for new projects") + " (F5)"} aria-label={t("Check the team for new projects")}><svg class="ico-s" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-2.64-6.36L21 8"/><path d="M21 3v5h-5"/></svg></button>
          {/if}
        </div>
        {#if current && overview.teamError}
          <div class="offline" title={overview.teamError}>● {t("Storage not reachable")}</div>
        {:else if current && !overview.teamChecked}
          <div class="offline checking">● {t("checking…")}</div>
        {/if}
        <ul>
          {#each entries as p (p.root || p.id)}
            {@render row(p)}
          {:else}
            <li class="empty faint">{t("No projects in this team yet.")}</li>
          {/each}
        </ul>
        <button class="add" onclick={addToTeam} disabled={busy === "add" || !current}>
          <span>+ {t("Add project")}</span>
          <span class="hint">{t("Select project folder")}</span>
        </button>
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
            {@const name = proj ? proj.name : t("New tab")}
            {@const team = teamOf(x.team)}
            <div class="tab" class:on={activeTab(x)} onpointerdown={(e) => tabDown(e, x.id)}>
              <button class="tab-name" role="tab" aria-selected={activeTab(x)}
                title={proj ? tabTitle(team?.name ?? "", proj) : name}
                onclick={() => goTab(x)}>
                {#if proj}<ProjectIcon p={proj} size={16} />{:else}<span class="tab-icon" aria-hidden="true">+</span>{/if}{name}
                {#if manyTeams && team}<span class="tab-team" style:--c={cssColor(pickFor(team.id))} aria-hidden="true">{initial(team.name)}</span>{/if}
              </button>
              <button class="tab-x" onclick={() => closeTabOf(x)} aria-label={t("Close {name}", { name })}
                title={activeTab(x) ? t("Close the tab") + " (Ctrl+W)" : t("Close the tab")}>×</button>
            </div>
          {/each}
          <button class="tab-add" onclick={newTab} aria-label={t("New tab")} title={t("New tab (Ctrl+T)")}>+</button>
        </div>
        {@render winControls()}
      </div>
      <div class="content-body">
      {#if blank}
        <NewTab {overview} projects={entries} open={isOpen} {reload} onopen={openHere}
          onadd={addToTeam} adding={busy === "add"} />
      {:else if selectedEntry && selectedEntry.status === "downloaded"}
        {#key selectedEntry.root}
          <ProjectView root={selectedEntry.root} {refreshKey} teams={overview.teams} onchanged={reload}
            onsettings={() => (settingsFor = selectedEntry ?? null)}
            entry={selectedEntry} onteamsettings={(team) => (teamSettingsFor = team)}
            settings={settingsTab}
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
    <button class="ghost" onclick={() => openLink(u.pageUrl)}>{t("What's new")}</button>
  </div>
  {#if updState.stage === "failed"}<p class="upd-error">{updState.error}</p>{/if}
  {#if u.installable && updState.stage === "ready" && updState.auto && !u.required}
    <p class="faint small upd-note">{t("Or it installs by itself when R3V is in the tray.")}</p>
  {/if}
{/snippet}

{#snippet row(p: TeamProject)}
  <li>
    <button class="proj {p.status}" class:on={!blank && selectedEntry === p} onclick={() => select(p)}
      class:locked={p.status === "remote" && teamLocked}
      title={p.status === "remote" ? t("On the team, not on this computer yet") : p.root}>
      <ProjectIcon {p} size={24} />
      <span class="text">
        <span class="name">{p.name}{#if pinned.includes(rowKey(p))}<svg class="pin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-label={t("Pinned")}><title>{t("Pinned")}</title><path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/></svg>{/if}</span>
        <span class="meta" class:busy={p.root && activity[p.root]}>
          {p.root && activity[p.root] ? progressShort(activity[p.root]) : statusText(p.status) ?? `⑂ ${p.branchLabel || p.branch}`}
        </span>
      </span>
    </button>
    {#if p.root && preuploads[p.root]}<span class="pre-slot"><PreuploadIcon p={preuploads[p.root]} /></span>{/if}
    <button class="ghost more" class:open={rowMenu === rowKey(p)} title={t("More")}
      onclick={() => toggleMenu(p)}>⋯</button>
    {#if rowMenu === rowKey(p)}
      <div class="row-menu surface-menu" role="menu">
        <button class="item" onclick={() => { rowMenu = ""; togglePin(p); }}>{pinned.includes(rowKey(p)) ? t("Unpin") : t("Pin to top")}</button>
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
        <div class="sep"></div>
        <button class="item" onclick={() => { rowMenu = ""; settingsFor = p; }}>{t("Settings…")}</button>
      </div>
    {/if}
  </li>
{/snippet}


<Tooltip />
{#if keysHelp}<KeysHelp onclose={() => (keysHelp = false)} />{/if}
{#if queue.open}
  <UploadQueue names={Object.fromEntries(entries.filter((p) => p.root).map((p) => [p.root, p.name]))} onclose={() => (queue.open = false)} />
{/if}

{#if appSettings}
  <AppSettings version={appVersion} {edition} {autostart} autoUpdate={updState.auto} {downloadDir} {update}
    checking={checkingNow} onautostart={toggleAutostart} onautoupdate={setAutoUpdate} oncheck={checkUpdateNow}
    ondownloaddir={changeDownloadDir} onupdate={(u) => (update = u)} onclose={() => (appSettings = false)} />
{/if}

{#if userSettings}
  <UserSettings team={current} onclose={() => (userSettings = false)}
    onchanged={async (p) => { profile = p; await reload(); refreshKey++; }} />
{/if}

{#snippet projectSettings(p: TeamProject, inline: boolean)}
  <ProjectSettings {p} team={current} {inline}
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
  .shell { display: grid; grid-template-columns: 250px 1fr; height: 100%; }
  .shell.folded { grid-template-columns: 52px 1fr; }
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
  .folded-top { flex-direction: column; margin-left: calc(var(--sp-6) * -1); margin-right: calc(var(--sp-6) * -1); }
  .folded-top .fold { margin: 0; }
  .folded-list { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; align-items: center; gap: var(--sp-6);
    padding: var(--sp-10) 0; margin: 0 calc(var(--sp-6) * -1); }
  .folded-proj { padding: var(--sp-4); border: var(--border-width) solid transparent; border-radius: var(--radius); background: transparent; line-height: 0; }
  .folded-proj:hover:not(:disabled) { background: var(--panel); border-color: transparent; }
  .folded-proj.on { background: var(--panel-2); border-color: var(--line-strong); }
  .add-icon { width: 38px; height: 38px; line-height: 1; font-size: var(--fs-lg); color: var(--muted); border-style: dashed; border-color: var(--line); }
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
  .proj { flex: 1; min-width: 0; display: flex; align-items: center; gap: var(--sp-10); border: none; background: transparent; padding: var(--sp-6) var(--sp-28) var(--sp-6) var(--sp-10); border-radius: var(--radius-lg); text-align: left; }
  .proj:hover { background: var(--panel); }
  .proj.on { background: var(--panel-2); }
  .icon { width: 16px; text-align: center; color: var(--accent); }
  .text { display: flex; flex-direction: column; min-width: 0; }
  .name { font-weight: var(--fw-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { font-size: var(--fs-sm); color: var(--faint); }
  /* Not on this computer: dimmed, with a cloud icon (not colour alone). */
  .proj.remote .name { color: var(--muted); font-weight: var(--fw-medium); }
  .proj.remote .icon { color: var(--faint); }
  .proj.locked { opacity: .6; }
  .proj.missing .icon, .proj.missing .meta { color: var(--warn); }
  .more {
    position: absolute; top: 4px; right: 4px; visibility: hidden; padding: 0 var(--sp-6); line-height: 18px;
    font-size: var(--fs-lg); color: var(--muted); border-radius: var(--radius);
  }
  li:hover .more, .more.open { visibility: visible; }
  /* (beside the project's button, not in it: a button of its own) */
  .pre-slot { position: absolute; top: 9px; right: 28px; }
  li:hover .pre-slot { right: 30px; }
  .more:hover, .more.open { background: var(--hover); color: var(--text); }
  .row-menu {
    position: absolute; top: 26px; right: 4px; z-index: var(--z-menu); min-width: 220px; padding: var(--sp-6);
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
  .pin { width: 11px; height: 11px; margin-left: var(--sp-4); color: var(--faint); vertical-align: -1px; flex: none; }
  .danger-text { color: var(--danger); }
  .confirm-input { width: 100%; margin-top: var(--sp-6); }
  .empty { padding: var(--sp-6) var(--sp-10); font-size: var(--fs-md); }
  .offline { font-size: var(--fs-sm); color: var(--danger); padding: 0 var(--sp-8) var(--sp-4); }
  .offline.checking { color: var(--faint); }
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
  .update { margin: 0 0 var(--sp-10); padding: var(--sp-8) var(--sp-10); border-radius: var(--radius-lg); background: var(--accent-bg); border: var(--border-width) solid var(--accent-line); font-size: var(--fs-md); }
  .update-h { display: flex; align-items: center; gap: var(--sp-6); font-weight: var(--fw-semibold); color: var(--accent); }
  .update-h span { flex: 1; }
  .update-a { display: flex; gap: var(--sp-6); margin-top: var(--sp-6); }
  .update-a button { padding: var(--sp-4) var(--sp-10); font-size: var(--fs-md); }
  .update-a { align-items: center; }
  .upd-progress { flex: 1; height: 5px; border-radius: var(--radius-xs); background: var(--accent-line); overflow: hidden; }
  .upd-progress span { display: block; height: 100%; background: var(--accent); transition: width .2s; }
  .upd-error { margin: var(--sp-6) 0 0; color: var(--danger); font-size: var(--fs-sm); }
  .upd-note { margin: var(--sp-4) 0 0; }
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
  .tab-icon { color: var(--accent); font-size: var(--fs-sm); }
  /* Which team a tab is on, once tabs of more than one are open. */
  .tab-team { flex: none; margin-left: auto; padding: 0 var(--sp-4); border-radius: var(--radius-pill); font-size: var(--fs-2xs);
    font-weight: var(--fw-semibold); line-height: 1.4; color: var(--c); background: color-mix(in srgb, var(--c) 16%, transparent); }
  .tab-x { border: none; background: transparent; padding: 0 var(--sp-8); color: var(--faint); font-size: var(--fs-lg); line-height: 1; visibility: hidden; }
  .tab:hover .tab-x, .tab.on .tab-x { visibility: visible; }
  .tab-x:hover:not(:disabled) { background: transparent; color: var(--text); }
  .tab-add { align-self: center; border: none; background: transparent; padding: 0 var(--sp-10); font-size: var(--fs-lg); color: var(--muted); }
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
  .keys-warn { margin: var(--sp-6) var(--sp-12) 0; padding: var(--sp-8) var(--sp-10); border-radius: var(--radius); font-size: var(--fs-sm); line-height: 1.4;
    background: var(--warn-bg); color: var(--warn); }
</style>
