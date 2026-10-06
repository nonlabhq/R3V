<script lang="ts">
  import { onMount } from "svelte";
  import { Events, Window } from "@wailsio/runtime";
  import { api, errorText, formatBytes, progressShort, type Overview, type Progress, type ProjectInfo, type TeamProject } from "./lib/api";
  import type { DownloadSize, UpdateInfo, UpdateState } from "../bindings/github.com/nonlabhq/r3v/desktop/models";
  import ProgressBar from "./lib/ProgressBar.svelte";
  import { toast } from "./lib/notify.svelte";
  import ProjectView from "./lib/ProjectView.svelte";
  import NewTab from "./lib/NewTab.svelte";
  import { untrack } from "svelte";
  import KeptSamples from "./lib/KeptSamples.svelte";
  import Onboarding from "./lib/Onboarding.svelte";
  import TeamMenu from "./lib/TeamMenu.svelte";
  import VerifyDialog from "./lib/VerifyDialog.svelte";
  import Modal from "./lib/Modal.svelte";
  import Toasts from "./lib/Toasts.svelte";
  import StyleLab from "./lib/StyleLab.svelte";
  import ProjectSettings from "./lib/ProjectSettings.svelte";
  import AppSettings from "./lib/AppSettings.svelte";
  import { t, tn } from "./lib/i18n.svelte";
  import PreuploadIcon from "./lib/PreuploadIcon.svelte";
  import { preuploads, watchPreuploads } from "./lib/preupload.svelte";

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
  let pinned = $state<string[]>((() => { try { return JSON.parse(localStorage.getItem(PINNED_KEY) ?? "[]"); } catch { return []; } })());
  function togglePin(p: TeamProject) {
    const key = rowKey(p);
    pinned = pinned.includes(key) ? pinned.filter((k) => k !== key) : [...pinned, key];
    try { localStorage.setItem(PINNED_KEY, JSON.stringify(pinned)); } catch { /* not remembered */ }
  }
  const DOWNLOAD_DIR_KEY = "r3v.downloadDir";
  const remember = (k: string, v: string) => { try { localStorage.setItem(k, v); } catch { /* not persisted */ } };
  const recall = (k: string) => { try { return localStorage.getItem(k) ?? ""; } catch { return ""; } };

  let reloading = $state(false);
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

  // Projects open as tabs at the top (per team, remembered; drag one to
  // move it). The sidebar opens one, or goes to it when it is open already.
  // A new tab ("new:…", Ctrl+T or +) picks a team and a project to open in
  // it; new tabs aren't remembered, and stay when the team changes.
  const tabsKey = () => `r3v.tabs:${overview?.currentTeam ?? ""}`;
  const isBlank = (k: string) => k.startsWith("new:");
  let tabKeys = $state<string[]>([]);
  let blank = $state(""); // the new tab shown ("" a project's)
  let tabsClosed = $state(false); // every tab closed: nothing is picked for you
  $effect.pre(() => {
    overview?.currentTeam; // the team's tabs
    let saved: string[] = [];
    try { saved = JSON.parse(localStorage.getItem(tabsKey()) ?? "[]"); } catch { /* none */ }
    tabKeys = [...saved, ...untrack(() => tabKeys).filter(isBlank)];
    tabsClosed = false;
  });
  type Tab = { key: string; p?: TeamProject };
  let tabItems = $derived(tabKeys.map((k): Tab | null => {
    if (isBlank(k)) return { key: k };
    const p = entries.find((e) => rowKey(e) === k);
    return p ? { key: k, p } : null;
  }).filter((x): x is Tab => !!x));
  let tabEntries = $derived(tabItems.flatMap((x) => (x.p ? [x.p] : [])));
  const activeTab = (x: Tab) => (x.p ? !blank && selectedEntry === x.p : blank === x.key);
  function saveTabs() { remember(tabsKey(), JSON.stringify(tabKeys.filter((k) => !isBlank(k)))); }
  function openTab(p: TeamProject) {
    tabsClosed = false;
    if (tabKeys.includes(rowKey(p))) return;
    tabKeys = [...tabKeys, rowKey(p)];
    saveTabs();
  }
  function newTab() {
    const key = `new:${Date.now()}`;
    tabKeys = [...tabKeys, key];
    blank = key;
    tabsClosed = false;
  }
  // From a new tab: the project takes its place (or its own tab, if open).
  function openHere(p: TeamProject) {
    const here = blank;
    if (tabKeys.includes(rowKey(p))) tabKeys = tabKeys.filter((k) => k !== here);
    else tabKeys = tabKeys.map((k) => (k === here ? rowKey(p) : k));
    saveTabs();
    select(p);
  }
  function closeTab(x: Tab) {
    const at = tabItems.findIndex((y) => y.key === x.key);
    const wasActive = activeTab(x);
    tabKeys = tabKeys.filter((k) => k !== x.key);
    saveTabs();
    if (!wasActive) return;
    const rest = tabItems.filter((y) => y.key !== x.key);
    const next = rest[Math.min(at, rest.length - 1)];
    if (next?.p) select(next.p);
    else if (next) blank = next.key;
    else { selected = {}; blank = ""; tabsClosed = true; }
  }
  // Dragging a tab moves it among the others (a drag isn't a click).
  let tabDrag: { key: string; x: number; moved: boolean } | null = null;
  let tabDragged = false;
  let tabsEl = $state<HTMLElement>();
  function tabDown(e: PointerEvent, key: string) {
    if (e.button !== 0 || (e.target as HTMLElement).closest(".tab-x")) return;
    tabDrag = { key, x: e.clientX, moved: false };
    const move = (m: PointerEvent) => {
      if (!tabDrag) return;
      if (!tabDrag.moved && Math.abs(m.clientX - tabDrag.x) < 5) return;
      tabDrag.moved = true;
      const els = [...(tabsEl?.querySelectorAll<HTMLElement>(".tab") ?? [])];
      let to = els.findIndex((el) => { const r = el.getBoundingClientRect(); return m.clientX < r.left + r.width / 2; });
      if (to < 0) to = els.length;
      const from = tabKeys.indexOf(tabDrag.key);
      if (to > from) to--;
      if (to !== from) {
        const keys = tabKeys.filter((k) => k !== tabDrag!.key);
        keys.splice(to, 0, tabDrag.key);
        tabKeys = keys;
      }
    };
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      if (tabDrag?.moved) { tabDragged = true; saveTabs(); setTimeout(() => (tabDragged = false)); }
      tabDrag = null;
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
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
    if (!overview || selectedEntry || tabsClosed || blank) return;
    const last = recall(SELECTED_KEY);
    const pick = entries.find((p) => p.root && p.root === last) ??
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
    return () => {
      offProgress();
      offUpdate();
      offWatch();
    };
  });

  const statusText = (status: string) => ({ remote: t("not downloaded"), missing: t("folder not found") } as Record<string, string>)[status];
  const statusIcon: Record<string, string> = { remote: "☁", missing: "⚠", downloaded: "♪" };
</script>

<svelte:window onfocus={reloadIfStale}
  onkeydown={(e) => {
    if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === "t" && overview && !onboarding) {
      e.preventDefault();
      newTab();
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
    <div class="splash-logo"><img src="/icon.png" alt="" />R3V</div>
    <div class="splash-band"></div>
  </div>
{:else if onboarding}
  <Onboarding {overview} {reload} onfinish={async (root, share) => {
    onboarding = false;
    if (root && share) firstShare = root;
    else if (root) justDownloaded = root;
    await reload();
    if (root) selected = { root };
  }} />
{:else}
  <div class="shell" class:folded>
    <aside>
      {#if folded}
      <div class="aside-top folded-top">
        <button class="logo" onclick={() => (appSettings = true)} title={t("R3V settings")} aria-label={t("R3V settings")}>
          <img src="/icon.png" alt="" />
        </button>
        <button class="ghost fold" onclick={() => fold(false)} title={t("Show the sidebar")} aria-label={t("Show the sidebar")}>»</button>
      </div>
      {:else}
      <div class="aside-top">
      <button class="brand" onclick={() => (appSettings = true)} title={t("R3V settings")}>
        <img src="/icon.png" alt="" /> R3V
        {#if edition}<span class="edition" title={t("A R3V build with extensions")}>{edition}</span>{/if}
        {#if appVersion}<span class="version faint">v{appVersion}</span>{/if}
      </button>
      <button class="ghost fold" onclick={() => fold(true)} title={t("Hide the sidebar")} aria-label={t("Hide the sidebar")}>«</button>
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
      <TeamMenu {overview} {reload} />
      {#if current?.keysUnreadable}
        <p class="keys-warn">{t("This computer can't read the keys of “{team}” (R3V's settings came from another computer or Windows user). Enter them again in the team's settings (⚙).", { team: current.name })}</p>
      {/if}

      <div class="list">
        <div class="section row-h">
          <span>{t("Projects")}</span>
          {#if current}
            <button class="ghost tiny" class:spin={reloading} onclick={reload} title={t("Check the team for new projects")} aria-label={t("Check the team for new projects")}><svg class="ico-s" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-2.64-6.36L21 8"/><path d="M21 3v5h-5"/></svg></button>
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
      {/if}

    </aside>

    <section class="content">
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="titlebar" ondblclick={(e) => { if (e.target === e.currentTarget) win(() => Window.ToggleMaximise()); }}>
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div class="tabs" role="tablist" tabindex="-1" aria-label={t("Open projects")} bind:this={tabsEl}
          onclickcapture={(e) => { if (tabDragged) { e.stopPropagation(); e.preventDefault(); } }}>
          {#each tabItems as x (x.key)}
            {@const name = x.p ? x.p.name : t("New tab")}
            <div class="tab" class:on={activeTab(x)} onpointerdown={(e) => tabDown(e, x.key)}>
              <button class="tab-name" role="tab" aria-selected={activeTab(x)} title={x.p ? x.p.root || x.p.name : name}
                onclick={() => (x.p ? select(x.p) : (blank = x.key))}>
                <span class="tab-icon" aria-hidden="true">{x.p ? statusIcon[x.p.status] : "+"}</span>{name}
              </button>
              <button class="tab-x" onclick={() => closeTab(x)} aria-label={t("Close {name}", { name })}>×</button>
            </div>
          {/each}
          <button class="tab-add" onclick={newTab} aria-label={t("New tab")} title={t("New tab (Ctrl+T)")}>+</button>
        </div>
        {@render winControls()}
      </div>
      <div class="content-body">
      {#if blank}
        <NewTab {overview} projects={entries} open={(p) => tabKeys.includes(rowKey(p))} {reload} onopen={openHere}
          onadd={addToTeam} adding={busy === "add"} />
      {:else if selectedEntry && selectedEntry.status === "downloaded"}
        {#key selectedEntry.root}
          <ProjectView root={selectedEntry.root} {refreshKey} teams={overview.teams} onchanged={reload}
            onsettings={() => (settingsFor = selectedEntry ?? null)}
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
    <button class="proj {p.status}" class:on={selectedEntry === p} onclick={() => select(p)}
      class:locked={p.status === "remote" && teamLocked}
      title={p.status === "remote" ? t("On the team, not on this computer yet") : p.root}>
      <span class="icon" aria-hidden="true">{statusIcon[p.status]}</span>
      <span class="text">
        <span class="name">{p.name}{#if pinned.includes(rowKey(p))}<svg class="pin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-label={t("Pinned")}><title>{t("Pinned")}</title><path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/></svg>{/if}{#if p.root && preuploads[p.root]}<PreuploadIcon p={preuploads[p.root]} />{/if}</span>
        <span class="meta" class:busy={p.root && activity[p.root]}>
          {p.root && activity[p.root] ? progressShort(activity[p.root]) : statusText(p.status) ?? `⑂ ${p.branch}`}
        </span>
      </span>
    </button>
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


{#if appSettings}
  <AppSettings version={appVersion} {edition} {autostart} autoUpdate={updState.auto} {downloadDir} {update}
    checking={checkingNow} onautostart={toggleAutostart} onautoupdate={setAutoUpdate} oncheck={checkUpdateNow}
    ondownloaddir={changeDownloadDir} onupdate={(u) => (update = u)} onclose={() => (appSettings = false)} />
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
  .shell.folded aside { padding-left: var(--sp-6); padding-right: var(--sp-6); }
  .aside-top { display: flex; align-items: center; gap: var(--sp-4); }
  .aside-top .brand { flex: 1; min-width: 0; }
  .fold { flex: none; padding: var(--sp-2) var(--sp-8); color: var(--faint); font-size: var(--fs-lg); line-height: 1; margin-bottom: var(--sp-8); }
  .fold:hover:not(:disabled) { color: var(--text); }
  .folded-top { flex-direction: column; margin-left: calc(var(--sp-6) * -1); margin-right: calc(var(--sp-6) * -1); }
  .folded-top .fold { margin: 0; }
  .logo { border: none; background: transparent; padding: var(--sp-6); border-radius: var(--radius); }
  .logo img { width: 22px; height: 22px; display: block; }
  aside { background: var(--bg-sunken); border-right: var(--border-width) solid var(--line); display: flex; flex-direction: column; padding: var(--sp-12) var(--sp-10); min-height: 0; }
  .brand { display: flex; align-items: center; gap: var(--sp-8); font-weight: var(--fw-bold); font-size: var(--fs-lg); padding: var(--sp-4) var(--sp-8); margin: calc(var(--sp-2) * -1) 0 var(--sp-8);
    background: none; border: 0; border-radius: var(--radius); color: var(--text); text-align: left; cursor: pointer; width: 100%; }
  .brand:hover { background: var(--panel-2); }
  .brand img { width: 20px; height: 20px; }
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
  .version { margin-left: auto; font-size: var(--fs-xs); font-weight: 400; }
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
